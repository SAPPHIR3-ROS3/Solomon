package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

const anthropicAPIVersion = "2023-06-01"

type AnthropicBackend struct {
	baseURL string
	apiKey  string
}

func NewAnthropicBackend(baseURL, apiKey string) *AnthropicBackend {
	return &AnthropicBackend{baseURL: strings.TrimSpace(baseURL), apiKey: strings.TrimSpace(apiKey)}
}

func (b *AnthropicBackend) Protocol() Protocol { return ProtocolAnthropic }

func (b *AnthropicBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	body := b.buildSimpleBody(req, false)
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := anthropicHTTPNew(ctx, anthropicMessagesURL(b.baseURL), raw, b.apiKey)
	if err != nil {
		return "", err
	}
	resp, err := anthropicHTTPDefault().Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", fmt.Errorf("anthropic API: %s: %s", resp.Status, strings.TrimSpace(string(bb)))
	}
	var msg struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return "", err
	}
	var parts []string
	for _, c := range msg.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (b *AnthropicBackend) ListModels(ctx context.Context) ([]string, error) {
	_ = ctx
	return modelsapi.CuratedAnthropicModels(), nil
}
