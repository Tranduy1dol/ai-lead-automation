package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

func TestDiscordNotifyPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request = %s, content type = %q", r.Method, r.Header.Get("Content-Type"))
		}
		if r.URL.Query().Get("wait") != "true" || r.URL.Query().Get("thread_id") != "123" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		var payload struct {
			Embeds []struct {
				Fields []struct{ Name, Value string }
			} `json:"embeds"`
			AllowedMentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if len(payload.Embeds) != 1 || len(payload.Embeds[0].Fields) != 6 {
			t.Errorf("payload = %#v", payload)
			return
		}
		want := map[string]string{"Lead ID": "42", "Company": "Acme @everyone", "Category": "sales", "Priority": "high", "Classification summary": "Needs automation", "Recommended action": "Schedule a call"}
		for _, field := range payload.Embeds[0].Fields {
			if field.Value != want[field.Name] {
				t.Errorf("field %q = %q", field.Name, field.Value)
			}
			delete(want, field.Name)
		}
		if len(want) != 0 {
			t.Errorf("missing fields: %v", want)
		}
		if payload.AllowedMentions.Parse == nil || len(payload.AllowedMentions.Parse) != 0 {
			t.Error("mentions must be explicitly disabled")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	notifier, err := NewDiscordNotifier(server.URL + "?thread_id=123&wait=false")
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.Notify(context.Background(), lead.Lead{ID: 42, Company: "Acme @everyone", Classification: lead.Classification{Category: "sales", Priority: "high", Summary: "Needs automation", RecommendedAction: "Schedule a call"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiscordNotifyStatusWithoutRetryOrRedirect(t *testing.T) {
	for _, status := range []int{200, 204, 302, 307, 400, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("secret-response-body"))
			}))
			defer server.Close()
			notifier, err := NewDiscordNotifier(server.URL + "/secret-token")
			if err != nil {
				t.Fatal(err)
			}
			err = notifier.Notify(context.Background(), lead.Lead{})
			if status < 300 && err != nil {
				t.Fatal(err)
			}
			if status >= 300 && (err == nil || !strings.Contains(err.Error(), fmt.Sprint(status))) {
				t.Fatalf("error = %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("secret leaked: %v", err)
			}
			if calls.Load() != 1 {
				t.Errorf("calls = %d, want 1", calls.Load())
			}
		})
	}
}

func TestDiscordNotifyCancellationAndTimeout(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-release
	}))
	defer server.Close()
	defer close(release)
	notifier, err := NewDiscordNotifier(server.URL + "/secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if notifier.client.Timeout != 5*time.Second {
		t.Fatalf("HTTP timeout = %v", notifier.client.Timeout)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := notifier.Notify(ctx, lead.Lead{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("canceled request reached server")
	}
	for _, timeout := range []time.Duration{50 * time.Millisecond, 0} {
		ctx := context.Background()
		cancel := func() {}
		if timeout != 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		}
		start := time.Now()
		err := notifier.Notify(ctx, lead.Lead{})
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout error = %v", err)
		}
		if strings.Contains(err.Error(), "secret-token") {
			t.Fatalf("URL leaked: %v", err)
		}
		if time.Since(start) > 7*time.Second {
			t.Fatal("timeout not bounded")
		}
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestDiscordConfiguration(t *testing.T) {
	for _, raw := range []string{"", "/relative", "ftp://example.com/webhook", "https://", "https://user:secret@example.com", "https://example.com/#secret", "://secret"} {
		if _, err := NewDiscordNotifier(raw); err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("URL %q: error = %v", raw, err)
		}
	}
}

func TestFieldValue(t *testing.T) {
	for _, value := range []string{"", "  ", "Acme", strings.Repeat("a", 901), strings.Repeat("界🙂", 400)} {
		got := fieldValue(value)
		if got == "" || len(got) > 900 || !utf8.ValidString(got) {
			t.Errorf("invalid field: length %d", len(got))
		}
		if value == "Acme" && got != value {
			t.Errorf("short value changed: %q", got)
		}
		if len(value) > 900 && !strings.HasSuffix(got, "…") {
			t.Error("missing truncation marker")
		}
	}
}
