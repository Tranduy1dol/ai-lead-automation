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
	"github.com/Tranduy1dol/ai-lead-automation/internal/llm"
)

func TestRunRequiresOpenAIConfigurationBeforeDatabaseConnection(t *testing.T) {
	for _, tt := range []struct{ key, model, want string }{
		{"", "test-model", "OPENAI_API_KEY"},
		{"test-key", "", "OPENAI_MODEL"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			t.Setenv("CLASSIFIER_MODE", "openai")
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
	t.Setenv("CLASSIFIER_MODE", "openai")
	t.Setenv("DATABASE_URL", "invalid-database-url")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("DISCORD_WEBHOOK_URL", "invalid-webhook-url")
	err := run(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "DISCORD_WEBHOOK_URL") {
		t.Fatalf("error = %v, want invalid Discord configuration", err)
	}
}

func TestMockClassifier(t *testing.T) {
	t.Setenv("CLASSIFIER_MODE", "mock")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")
	classifier, err := newClassifier()
	if err != nil {
		t.Fatal(err)
	}
	want := lead.Classification{Category: "sales", Priority: "normal", Summary: "Demo lead received.", RecommendedAction: "Review the lead and contact the prospect."}
	for _, value := range []lead.Lead{{}, {Company: "Acme", Message: "Hello"}} {
		got, err := classifier.Classify(context.Background(), value)
		if err != nil || got != want {
			t.Fatalf("classification = %#v, error = %v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := classifier.Classify(ctx, lead.Lead{}); err != context.Canceled {
		t.Fatalf("error = %v", err)
	}
}

func TestUnknownClassifierMode(t *testing.T) {
	t.Setenv("CLASSIFIER_MODE", "typo")
	if _, err := newClassifier(); err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestOpenRouterMode(t *testing.T) {
	t.Setenv("CLASSIFIER_MODE", "openrouter")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("OPENROUTER_MODEL", "")
	classifier, err := newClassifier()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := classifier.(*llm.OpenRouterClassifier); !ok {
		t.Fatalf("classifier = %T", classifier)
	}
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("DATABASE_URL", "invalid-database-url")
	if err := run(slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenAIModeCompatibility(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("OPENROUTER_API_KEY", "")
	for _, mode := range []string{"", "openai"} {
		t.Setenv("CLASSIFIER_MODE", mode)
		classifier, err := newClassifier()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := classifier.(*llm.OpenAIClassifier); !ok {
			t.Fatalf("classifier = %T", classifier)
		}
	}
}
