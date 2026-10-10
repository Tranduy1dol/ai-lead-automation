package lead_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
	"github.com/Tranduy1dol/ai-lead-automation/migration"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepository(t *testing.T) {
	if os.Getenv("RUN_POSTGRES_TESTS") != "1" {
		t.Skip("set RUN_POSTGRES_TESTS=1 to run with a disposable Docker PostgreSQL container")
	}
	// Never read DATABASE_URL: tests can only reach the container they create.
	docker := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			t.Fatalf("docker %v: %v: %s %s", args, err, output, stderr.String())
		}
		return strings.TrimSpace(string(output))
	}
	id := docker("run", "--detach", "--rm", "--publish", "127.0.0.1::5432",
		"--tmpfs", "/var/lib/postgresql/data", "--env", "POSTGRES_PASSWORD=isolated-test-only",
		"--env", "POSTGRES_DB=lead_test", "postgres:17.6-alpine")
	t.Cleanup(func() { docker("rm", "--force", id) })
	address := docker("port", id, "5432/tcp")
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://postgres:isolated-test-only@%s/lead_test?sslmode=disable", address))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PostgreSQL readiness: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
	}
	// A failed migration must leave neither partial DDL nor migration history.
	if _, err := pool.Exec(ctx, "CREATE TABLE leads (id BIGINT)"); err != nil {
		t.Fatal(err)
	}
	if err := migration.Apply(ctx, pool); err == nil {
		t.Fatal("expected conflicting schema error")
	}
	var historyExists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&historyExists); err != nil {
		t.Fatal(err)
	}
	if historyExists {
		t.Fatal("failed migration committed partial history")
	}
	if _, err := pool.Exec(ctx, "DROP TABLE leads"); err != nil {
		t.Fatal(err)
	}
	// Concurrent runs must apply each migration exactly once.
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- migration.Apply(ctx, pool) }()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	repository := lead.NewPostgresRepository(pool)
	input := lead.Lead{Name: "Demo", Email: "demo@example.com", Company: "Acme", Message: "Hello 世界",
		Classification: lead.Classification{Category: "sales", Priority: "normal", Summary: "Demo inquiry", RecommendedAction: "Contact prospect"}}
	created, err := repository.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 0 || created.CreatedAt.IsZero() {
		t.Fatalf("missing generated fields: %#v", created)
	}
	input.ID, input.CreatedAt = created.ID, created.CreatedAt
	if created != input {
		t.Fatalf("created = %#v, want %#v", created, input)
	}
	if err := migration.Apply(ctx, pool); err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}
	var stored lead.Lead
	err = pool.QueryRow(ctx, `SELECT id, name, email, company, message, category, priority, summary, recommended_action, created_at FROM leads WHERE id = $1`, created.ID).Scan(
		&stored.ID, &stored.Name, &stored.Email, &stored.Company, &stored.Message,
		&stored.Classification.Category, &stored.Classification.Priority, &stored.Classification.Summary,
		&stored.Classification.RecommendedAction, &stored.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if stored != created {
		t.Fatalf("stored = %#v, want %#v", stored, created)
	}
	t.Run("constraint error", func(t *testing.T) {
		invalid := input
		invalid.Name = strings.Repeat("x", 201)
		got, err := repository.Create(ctx, invalid)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "22001" || got != (lead.Lead{}) {
			t.Fatalf("lead = %#v, error = %v; want wrapped PostgreSQL length error", got, err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM leads").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("failed insert changed row count: %d", count)
		}
	})
	t.Run("canceled query", func(t *testing.T) {
		canceled, stop := context.WithCancel(ctx)
		stop()
		if _, err := repository.Create(canceled, input); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("closed database", func(t *testing.T) {
		pool.Close()
		if got, err := repository.Create(ctx, input); err == nil || got != (lead.Lead{}) {
			t.Fatalf("lead = %#v, error = %v", got, err)
		}
	})
}
