package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

func TestRunRequiresOpenAIConfigurationBeforeDatabaseConnection(t *testing.T) {
	for _, tt := range []struct{ key, model, want string }{
		{"", "test-model", "OPENAI_API_KEY"},
		{"test-key", "", "OPENAI_MODEL"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "invalid-database-url")
			t.Setenv("OPENAI_API_KEY", tt.key)
			t.Setenv("OPENAI_MODEL", tt.model)
			err := run(slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want missing %s", err, tt.want)
			}
		})
	}
}

type unusedCreator struct{}

func (unusedCreator) Create(context.Context, lead.CreateParams) (lead.Lead, error) {
	panic("unexpected call")
}

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := lead.NewHandler(unusedCreator{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	newMux(handler).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := recorder.Body.String(); got != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", got, `{"status":"ok"}`)
	}
}

func TestRunRejectsInvalidDiscordURLBeforeDatabaseConnection(t *testing.T) {
	t.Setenv("DATABASE_URL", "invalid-database-url")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("DISCORD_WEBHOOK_URL", "invalid-webhook-url")
	err := run(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "DISCORD_WEBHOOK_URL") {
		t.Fatalf("error = %v, want invalid Discord configuration", err)
	}
}
