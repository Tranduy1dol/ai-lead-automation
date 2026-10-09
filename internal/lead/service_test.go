package lead_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

type repositoryFunc func(context.Context, lead.Lead) (lead.Lead, error)

func (f repositoryFunc) Create(ctx context.Context, value lead.Lead) (lead.Lead, error) {
	return f(ctx, value)
}

func TestServiceCreateClassifiesAndPersistsLead(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	classification := lead.Classification{
		Category:          "sales",
		Priority:          "high",
		Summary:           "Invoice automation inquiry",
		RecommendedAction: "Schedule a call",
	}
	classifier := lead.ClassifierFunc(func(gotCtx context.Context, got lead.Lead) (lead.Classification, error) {
		if gotCtx != ctx {
			t.Fatal("classifier did not receive request context")
		}
		if got.Name != "John" || got.Email != "john@example.com" || got.Company != "Acme" || got.Message != "Help" {
			t.Fatalf("classifier lead = %#v", got)
		}
		return classification, nil
	})
	createdAt := time.Date(2026, time.October, 7, 10, 0, 0, 0, time.UTC)
	repository := repositoryFunc(func(gotCtx context.Context, got lead.Lead) (lead.Lead, error) {
		if gotCtx != ctx {
			t.Fatal("repository did not receive request context")
		}
		if got.Classification != classification {
			t.Fatalf("classification = %#v, want %#v", got.Classification, classification)
		}
		got.ID = 1
		got.CreatedAt = createdAt
		return got, nil
	})

	created, err := lead.NewService(classifier, repository, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).Create(ctx, lead.CreateParams{
		Name: " John ", Email: " john@example.com ", Company: " Acme ", Message: " Help ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.CreatedAt != createdAt || created.Classification != classification {
		t.Fatalf("created lead = %#v", created)
	}
}

func TestServiceCreateReturnsClassifierFailure(t *testing.T) {
	wantErr := errors.New("classifier unavailable")
	classifier := lead.ClassifierFunc(func(context.Context, lead.Lead) (lead.Classification, error) {
		return lead.Classification{}, wantErr
	})
	repositoryCalled := false
	repository := repositoryFunc(func(context.Context, lead.Lead) (lead.Lead, error) {
		repositoryCalled = true
		return lead.Lead{}, nil
	})

	_, err := lead.NewService(classifier, repository, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).Create(context.Background(), validParams())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if repositoryCalled {
		t.Fatal("repository called after classifier failure")
	}
}

func TestServiceCreateReturnsRepositoryFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	classifier := lead.ClassifierFunc(func(context.Context, lead.Lead) (lead.Classification, error) {
		return lead.Classification{Category: "sales"}, nil
	})
	repository := repositoryFunc(func(context.Context, lead.Lead) (lead.Lead, error) {
		return lead.Lead{}, wantErr
	})

	_, err := lead.NewService(classifier, repository, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).Create(context.Background(), validParams())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestServiceCreateRejectsInvalidInputBeforeDependencies(t *testing.T) {
	called := false
	classifier := lead.ClassifierFunc(func(context.Context, lead.Lead) (lead.Classification, error) {
		called = true
		return lead.Classification{}, nil
	})
	repository := repositoryFunc(func(context.Context, lead.Lead) (lead.Lead, error) {
		called = true
		return lead.Lead{}, nil
	})

	_, err := lead.NewService(classifier, repository, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).Create(context.Background(), lead.CreateParams{})
	var validationErr lead.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Error() != "name is required" {
		t.Fatalf("error = %v, want name validation error", err)
	}
	if called {
		t.Fatal("dependency called for invalid input")
	}
}

func validParams() lead.CreateParams {
	return lead.CreateParams{Name: "John", Email: "john@example.com", Company: "Acme", Message: "Help"}
}

type notifierFunc func(context.Context, lead.Lead) error

func (f notifierFunc) Notify(ctx context.Context, value lead.Lead) error {
	return f(ctx, value)
}

func TestServiceNotificationAfterPersistence(t *testing.T) {
	for _, tt := range []struct {
		name                                               string
		classificationErr, persistenceErr, notificationErr error
	}{
		{name: "success"},
		{name: "classification failure", classificationErr: errors.New("classification failed")},
		{name: "persistence failure", persistenceErr: errors.New("persistence failed")},
		{name: "notification failure", notificationErr: errors.New("notification failed")},
		{name: "notification canceled", notificationErr: context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), struct{}{}, "request")
			classification := lead.Classification{Category: "sales", Priority: "high", Summary: "Inquiry", RecommendedAction: "Call"}
			classifier := lead.ClassifierFunc(func(context.Context, lead.Lead) (lead.Classification, error) {
				return classification, tt.classificationErr
			})
			persistCalls, notifyCalls := 0, 0
			var persisted lead.Lead
			repository := repositoryFunc(func(_ context.Context, value lead.Lead) (lead.Lead, error) {
				persistCalls++
				value.ID = 42
				persisted = value
				return value, tt.persistenceErr
			})
			notifier := notifierFunc(func(gotCtx context.Context, value lead.Lead) error {
				notifyCalls++
				if persistCalls != 1 || tt.persistenceErr != nil {
					t.Fatal("notification before successful persistence")
				}
				if gotCtx != ctx || value != persisted {
					t.Fatalf("notification did not receive context and persisted lead: %#v", value)
				}
				return tt.notificationErr
			})
			var logs strings.Builder
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			created, err := lead.NewService(classifier, repository, notifier, logger).Create(ctx, validParams())
			if tt.classificationErr != nil || tt.persistenceErr != nil {
				want := tt.classificationErr
				if want == nil {
					want = tt.persistenceErr
				}
				if !errors.Is(err, want) || notifyCalls != 0 {
					t.Fatalf("error = %v, notifications = %d", err, notifyCalls)
				}
				return
			}
			if err != nil || created != persisted || persistCalls != 1 || notifyCalls != 1 {
				t.Fatalf("created = %#v, error = %v, persistence = %d, notifications = %d", created, err, persistCalls, notifyCalls)
			}
			if tt.notificationErr != nil && (!strings.Contains(logs.String(), "lead notification failed") || !strings.Contains(logs.String(), "lead_id=42")) {
				t.Fatalf("missing notification failure log: %s", logs.String())
			}
		})
	}
}
