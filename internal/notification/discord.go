package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

const requestTimeout = 5 * time.Second

type DiscordNotifier struct {
	webhookURL string
	client     *http.Client
}

var _ lead.Notifier = (*DiscordNotifier)(nil)

func NewDiscordNotifier(webhookURL string) (*DiscordNotifier, error) {
	u, err := url.Parse(strings.TrimSpace(webhookURL))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return nil, errors.New("DISCORD_WEBHOOK_URL must be an absolute HTTP(S) URL without user info or fragment")
	}
	query := u.Query()
	query.Set("wait", "true")
	u.RawQuery = query.Encode()
	return &DiscordNotifier{
		webhookURL: u.String(),
		client: &http.Client{
			Timeout:       requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (n *DiscordNotifier) Notify(ctx context.Context, value lead.Lead) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	fields := []map[string]string{
		{"name": "Lead ID", "value": strconv.FormatInt(value.ID, 10)},
		{"name": "Company", "value": fieldValue(value.Company)},
		{"name": "Category", "value": fieldValue(value.Classification.Category)},
		{"name": "Priority", "value": fieldValue(value.Classification.Priority)},
		{"name": "Classification summary", "value": fieldValue(value.Classification.Summary)},
		{"name": "Recommended action", "value": fieldValue(value.Classification.RecommendedAction)},
	}
	body, err := json.Marshal(map[string]any{
		"embeds":           []any{map[string]any{"title": "New lead", "fields": fields}},
		"allowed_mentions": map[string]any{"parse": []string{}},
	})
	if err != nil {
		return fmt.Errorf("encode Discord notification: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return errors.New("create Discord request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		// HTTP errors include the webhook URL, which contains a secret token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("call Discord: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func fieldValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(not provided)"
	}
	// Bound each field and the total embed size, preserving UTF-8 when truncating.
	if len(value) > 900 {
		value = value[:897]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		value += "…"
	}
	return value
}
