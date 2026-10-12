package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Tranduy1dol/ai-lead-automation/internal/lead"
)

type OpenRouterClassifier struct {
	apiKey string
	model  string
	client *http.Client
}

var _ lead.Classifier = (*OpenRouterClassifier)(nil)

func NewOpenRouterClassifier(apiKey, model string, client *http.Client) (*OpenRouterClassifier, error) {
	apiKey, model = strings.TrimSpace(apiKey), strings.TrimSpace(model)
	if apiKey == "" {
		return nil, errors.New("OPENROUTER_API_KEY is required")
	}
	if model == "" {
		model = "openrouter/free"
	}
	if client == nil {
		client = &http.Client{}
	}
	// Do not forward credentials or lead data through provider redirects.
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &OpenRouterClassifier{apiKey: apiKey, model: model, client: &copy}, nil
}

func (c *OpenRouterClassifier) Classify(ctx context.Context, value lead.Lead) (lead.Classification, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return lead.Classification{}, err
	}
	input, err := json.Marshal(map[string]string{"company": value.Company, "message": value.Message})
	if err != nil {
		return lead.Classification{}, fmt.Errorf("encode lead: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": classificationInstructions},
			{"role": "user", "content": string(input)},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "lead_classification", "strict": true,
				"schema": json.RawMessage(classificationSchema),
			},
		},
		"provider": map[string]bool{"require_parameters": true, "allow_fallbacks": false},
	})
	if err != nil {
		return lead.Classification{}, errors.New("encode OpenRouter request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return lead.Classification{}, errors.New("create OpenRouter request")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return lead.Classification{}, openRouterIOError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return lead.Classification{}, fmt.Errorf("OpenRouter API returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return lead.Classification{}, openRouterIOError(err)
	}
	if err := ctx.Err(); err != nil {
		return lead.Classification{}, err
	}
	if len(data) > maxResponseBytes {
		return lead.Classification{}, errors.New("OpenRouter response exceeds size limit")
	}
	var response struct {
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			FinishReason string          `json:"finish_reason"`
			Error        json.RawMessage `json:"error"`
			Message      struct {
				Role      string            `json:"role"`
				Content   string            `json:"content"`
				Refusal   string            `json:"refusal"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return lead.Classification{}, errors.New("malformed OpenRouter response")
	}
	if (len(response.Error) > 0 && string(response.Error) != "null") || len(response.Choices) != 1 {
		return lead.Classification{}, errors.New("OpenRouter response must contain one successful classification")
	}
	choice := response.Choices[0]
	if choice.FinishReason != "stop" || (len(choice.Error) > 0 && string(choice.Error) != "null") ||
		choice.Message.Role != "assistant" || choice.Message.Refusal != "" ||
		len(choice.Message.ToolCalls) != 0 || strings.TrimSpace(choice.Message.Content) == "" {
		return lead.Classification{}, errors.New("OpenRouter completion is invalid or incomplete")
	}
	return parseClassification(choice.Message.Content)
}

func openRouterIOError(err error) error {
	// Transport/body errors may include credentials; preserve only cancellation and timeout identity.
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return context.DeadlineExceeded
	}
	return errors.New("OpenRouter request or response I/O failed")
}
