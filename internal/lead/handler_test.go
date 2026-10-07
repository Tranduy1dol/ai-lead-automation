package lead_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

type creatorFunc func(context.Context, lead.CreateParams) (lead.Lead, error)

func (f creatorFunc) Create(ctx context.Context, params lead.CreateParams) (lead.Lead, error) {
	return f(ctx, params)
}

func TestCreate(t *testing.T) {
	createdAt := time.Date(2026, time.October, 6, 10, 0, 0, 0, time.UTC)
	creator := creatorFunc(func(ctx context.Context, params lead.CreateParams) (lead.Lead, error) {
		if ctx == nil {
			t.Fatal("context was not propagated")
		}
		want := lead.CreateParams{
			Name:    " John ",
			Email:   "john@example.com",
			Company: "Acme Inc",
			Message: "We need to automate invoice processing.",
		}
		if params != want {
			t.Fatalf("params = %#v, want %#v", params, want)
		}
		return lead.Lead{
			ID:      1,
			Name:    "John",
			Email:   params.Email,
			Company: params.Company,
			Message: params.Message,
			Classification: lead.Classification{
				Category:          "sales",
				Priority:          "high",
				Summary:           "Invoice automation inquiry",
				RecommendedAction: "Schedule a call",
			},
			CreatedAt: createdAt,
		}, nil
	})
	recorder := request(t, creator, `{
		"name":" John ",
		"email":"john@example.com",
		"company":"Acme Inc",
		"message":"We need to automate invoice processing."
	}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	want := `{"id":1,"name":"John","email":"john@example.com","company":"Acme Inc","message":"We need to automate invoice processing.","classification":{"category":"sales","priority":"high","summary":"Invoice automation inquiry","recommended_action":"Schedule a call"},"created_at":"2026-10-06T10:00:00Z"}` + "\n"
	if got := recorder.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	creator := creatorFunc(func(context.Context, lead.CreateParams) (lead.Lead, error) {
		return lead.Lead{}, lead.ValidationError("email must be valid")
	})
	recorder := request(t, creator, `{"name":"John","email":"not-an-email","company":"Acme","message":"Hello"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); got != "{\"error\":\"email must be valid\"}\n" {
		t.Errorf("body = %q", got)
	}
}

func TestCreateHidesPersistenceErrors(t *testing.T) {
	creator := creatorFunc(func(context.Context, lead.CreateParams) (lead.Lead, error) {
		return lead.Lead{}, errors.New("secret database detail")
	})
	recorder := request(t, creator, `{"name":"John","email":"john@example.com","company":"Acme","message":"Hello"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if got := recorder.Body.String(); got != "{\"error\":\"internal server error\"}\n" {
		t.Errorf("body = %q", got)
	}
	if strings.Contains(recorder.Body.String(), "database") {
		t.Errorf("body leaked internal error: %q", recorder.Body.String())
	}
}

func request(t *testing.T, creator lead.Creator, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := lead.NewHandler(creator, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	handler.Create(recorder, httptest.NewRequest(http.MethodPost, "/leads", strings.NewReader(body)))
	return recorder
}
