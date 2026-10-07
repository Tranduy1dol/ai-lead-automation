package lead_test

import (
	"context"
	"errors"
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

	created, err := lead.NewService(classifier, repository).Create(ctx, lead.CreateParams{
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

	_, err := lead.NewService(classifier, repository).Create(context.Background(), validParams())
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

	_, err := lead.NewService(classifier, repository).Create(context.Background(), validParams())
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

	_, err := lead.NewService(classifier, repository).Create(context.Background(), lead.CreateParams{})
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
