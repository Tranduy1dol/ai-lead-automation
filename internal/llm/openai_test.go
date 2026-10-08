package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const validClassification = `{"category":"sales","priority":"high","summary":" Purchase inquiry ","recommended_action":" Schedule a call "}`

func completedResponse(text string) string {
	encoded, _ := json.Marshal(text)
	return `{"status":"completed","error":null,"output":[{"type":"reasoning"},{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":` + string(encoded) + `}]}]}`
}

func newTestClassifier(t *testing.T, transport roundTripFunc) *OpenAIClassifier {
	t.Helper()
	c, err := NewOpenAIClassifier("test-key", "test-model", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassifyRequestAndSuccess(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader(completedResponse(validClassification))}
	c := newTestClassifier(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.String() != "https://api.openai.com/v1/responses" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("missing request headers")
		}
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > requestTimeout {
			t.Fatal("request has no bounded timeout")
		}
		var request struct {
			Model        string `json:"model"`
			Store        *bool  `json:"store"`
			Instructions string `json:"instructions"`
			Input        string `json:"input"`
			Text         struct {
				Format struct {
					Type   string `json:"type"`
					Name   string `json:"name"`
					Strict bool   `json:"strict"`
					Schema struct {
						Type                 string                     `json:"type"`
						Properties           map[string]json.RawMessage `json:"properties"`
						Required             []string                   `json:"required"`
						AdditionalProperties *bool                      `json:"additionalProperties"`
					} `json:"schema"`
				} `json:"format"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "test-model" || request.Store == nil || *request.Store || request.Instructions == "" {
			t.Fatalf("invalid request settings: %+v", request)
		}
		var input map[string]string
		if err := json.Unmarshal([]byte(request.Input), &input); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(input, map[string]string{"company": "Acme", "message": "Need a demo"}) {
			t.Fatalf("input = %v", input)
		}
		format := request.Text.Format
		if format.Type != "json_schema" || !format.Strict || format.Name == "" || format.Schema.Type != "object" ||
			format.Schema.AdditionalProperties == nil || *format.Schema.AdditionalProperties {
			t.Fatalf("invalid structured output format: %+v", format)
		}
		wantFields := []string{"category", "priority", "summary", "recommended_action"}
		if !reflect.DeepEqual(format.Schema.Required, wantFields) || len(format.Schema.Properties) != 4 {
			t.Fatalf("invalid schema: %+v", format.Schema)
		}
		for _, name := range wantFields {
			var property struct {
				Type string   `json:"type"`
				Enum []string `json:"enum"`
			}
			if err := json.Unmarshal(format.Schema.Properties[name], &property); err != nil || property.Type != "string" {
				t.Fatalf("invalid property %s: %s", name, format.Schema.Properties[name])
			}
			if name == "category" && !reflect.DeepEqual(property.Enum, []string{"sales", "support", "spam", "other"}) {
				t.Fatalf("category enum = %v", property.Enum)
			}
			if name == "priority" && !reflect.DeepEqual(property.Enum, []string{"low", "normal", "high"}) {
				t.Fatalf("priority enum = %v", property.Enum)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})
	got, err := c.Classify(context.Background(), lead.Lead{
		Name: "Private", Email: "private@example.com", Company: "Acme", Message: "Need a demo",
	})
	want := lead.Classification{Category: "sales", Priority: "high", Summary: "Purchase inquiry", RecommendedAction: "Schedule a call"}
	if err != nil || got != want {
		t.Fatalf("Classify = %+v, %v; want %+v", got, err, want)
	}
	if !body.closed {
		t.Fatal("response body not closed")
	}
}

func TestClassifyRejectsBadResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, `{"error":{"message":"secret-key"}}`},
		{"rate limited", 429, `{"error":{"message":"secret-key"}}`},
		{"server error", 500, "secret-key"},
		{"empty body", 200, ""},
		{"invalid envelope", 200, "secret-key"},
		{"trailing envelope", 200, completedResponse(validClassification) + `{}`},
		{"null envelope", 200, "null"},
		{"incomplete", 200, strings.Replace(completedResponse(validClassification), "completed", "incomplete", 1)},
		{"error envelope", 200, strings.Replace(completedResponse(validClassification), `"error":null`, `"error":{"message":"secret-key"}`, 1)},
		{"no output", 200, `{"status":"completed","output":[]}`},
		{"wrong role", 200, strings.Replace(completedResponse(validClassification), "assistant", "user", 1)},
		{"incomplete message", 200, strings.Replace(completedResponse(validClassification), `"role":"assistant","status":"completed"`, `"role":"assistant","status":"incomplete"`, 1)},
		{"refusal", 200, strings.Replace(completedResponse(validClassification), "output_text", "refusal", 1)},
		{"unknown content", 200, strings.Replace(completedResponse(validClassification), "output_text", "unknown", 1)},
		{"multiple texts", 200, `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{}"},{"type":"output_text","text":"{}"}]}]}`},
		{"oversized", 200, strings.Repeat("x", maxResponseBytes+1)},
	}
	for _, text := range []string{
		"", "null", "[]", "{}", "secret-key", "```json\n" + validClassification + "\n```",
		validClassification + "{}",
		strings.Replace(validClassification, `"sales"`, `"unknown"`, 1),
		strings.Replace(validClassification, `"high"`, `"urgent"`, 1),
		strings.Replace(validClassification, `"high"`, `1`, 1),
		strings.Replace(validClassification, `"summary"`, `"unexpected"`, 1),
		strings.Replace(validClassification, `"summary":" Purchase inquiry ",`, "", 1),
		strings.Replace(validClassification, `" Purchase inquiry "`, `"  "`, 1),
		strings.Replace(validClassification, `" Schedule a call "`, `null`, 1),
		strings.Replace(validClassification, `}`, `,"extra":"secret-key"}`, 1),
	} {
		tests = append(tests, struct {
			name   string
			status int
			body   string
		}{"invalid classification " + text, 200, completedResponse(text)})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tt.body)}
			c := newTestClassifier(t, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.status, Body: body}, nil
			})
			got, err := c.Classify(context.Background(), lead.Lead{})
			if err == nil || got != (lead.Classification{}) {
				t.Fatalf("Classify = %+v, %v; want empty result and error", got, err)
			}
			if strings.Contains(err.Error(), "secret-key") {
				t.Fatalf("error leaked provider content: %v", err)
			}
			if !body.closed {
				t.Fatal("response body not closed")
			}
		})
	}
}

func TestClassifyTransportError(t *testing.T) {
	want := errors.New("connection failed")
	c := newTestClassifier(t, func(*http.Request) (*http.Response, error) { return nil, want })
	if _, err := c.Classify(context.Background(), lead.Lead{}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestClassifyCancellationAndTimeout(t *testing.T) {
	for _, mode := range []string{"already canceled", "in flight", "deadline", "client timeout", "body read"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := error(context.Canceled)
			if mode == "already canceled" {
				cancel()
			}
			if mode == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 10*time.Millisecond)
				defer stop()
				want = context.DeadlineExceeded
			}
			c := newTestClassifier(t, func(r *http.Request) (*http.Response, error) {
				if mode == "already canceled" {
					t.Fatal("transport called with canceled context")
				}
				if mode == "body read" {
					return &http.Response{StatusCode: 200, Body: &cancelBody{ctx: r.Context(), cancel: cancel}}, nil
				}
				if mode == "in flight" {
					cancel()
				}
				<-r.Context().Done()
				return nil, r.Context().Err()
			})
			if mode == "client timeout" {
				c.client.Timeout = 10 * time.Millisecond
				want = context.DeadlineExceeded
			}
			if _, err := c.Classify(ctx, lead.Lead{}); !errors.Is(err, want) {
				t.Fatalf("error = %v, want wrapped %v", err, want)
			}
		})
	}
}

func TestNewOpenAIClassifierRequiresConfiguration(t *testing.T) {
	for _, tt := range []struct{ key, model, want string }{
		{"", "model", "OPENAI_API_KEY"}, {" \n", "model", "OPENAI_API_KEY"},
		{"key", "", "OPENAI_MODEL"}, {"key", " \t", "OPENAI_MODEL"},
	} {
		if _, err := NewOpenAIClassifier(tt.key, tt.model, nil); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("error = %v, want %s", err, tt.want)
		}
	}
	c, err := NewOpenAIClassifier(" key ", " model ", nil)
	if err != nil || c.client == nil || c.apiKey != "key" || c.model != "model" {
		t.Fatalf("invalid default configuration: error = %v", err)
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type cancelBody struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (b *cancelBody) Read([]byte) (int, error) {
	b.cancel()
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (*cancelBody) Close() error { return nil }
