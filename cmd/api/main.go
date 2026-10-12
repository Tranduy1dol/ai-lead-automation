package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
	"github.com/Tranduy1dol/ai-lead-automation/internal/llm"
	"github.com/Tranduy1dol/ai-lead-automation/internal/notification"
	"github.com/jackc/pgx/v5/pgxpool"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	classifier, err := newClassifier()
	if err != nil {
		return fmt.Errorf("configure classifier: %w", err)
	}
	var notifier lead.Notifier
	if webhookURL := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")); webhookURL != "" {
		notifier, err = notification.NewDiscordNotifier(webhookURL)
		if err != nil {
			return fmt.Errorf("configure notifier: %w", err)
		}
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(connectCtx, databaseURL)
	if err != nil {
		return fmt.Errorf("configure database pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(connectCtx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	leadService := lead.NewService(classifier, lead.NewPostgresRepository(pool), notifier, logger)
	leadHandler := lead.NewHandler(leadService, logger)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newMux(leadHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", "address", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}

		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		logger.Info("HTTP server stopped")
	}
	return nil
}

func newMux(leadHandler *lead.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("POST /leads", leadHandler.Create)
	return mux
}

func newClassifier() (lead.Classifier, error) {
	switch os.Getenv("CLASSIFIER_MODE") {
	case "", "openai":
		return llm.NewOpenAIClassifier(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"), nil)
	case "openrouter":
		return llm.NewOpenRouterClassifier(os.Getenv("OPENROUTER_API_KEY"), os.Getenv("OPENROUTER_MODEL"), nil)
	case "mock":
		return lead.ClassifierFunc(func(ctx context.Context, _ lead.Lead) (lead.Classification, error) {
			if err := ctx.Err(); err != nil {
				return lead.Classification{}, err
			}
			return lead.Classification{
				Category: "sales", Priority: "normal",
				Summary: "Demo lead received.", RecommendedAction: "Review the lead and contact the prospect.",
			}, nil
		}), nil
	default:
		return nil, errors.New("CLASSIFIER_MODE must be openai, openrouter, or mock")
	}
}
