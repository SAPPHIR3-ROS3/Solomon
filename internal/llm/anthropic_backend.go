package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

const AnthropicAPIVersion = "2023-06-01"

type AnthropicBackend struct {
	baseURL    string
	auth       AnthropicAuth
	httpClient *http.Client
}

func NewAnthropicBackend(baseURL string, auth AnthropicAuth) *AnthropicBackend {
	return NewAnthropicBackendWithClient(baseURL, auth, nil)
}

func NewAnthropicBackendWithClient(baseURL string, auth AnthropicAuth, client *http.Client) *AnthropicBackend {
	if client == nil {
		client = anthropicHTTPDefault()
	}
	return &AnthropicBackend{baseURL: strings.TrimSpace(baseURL), auth: auth, httpClient: client}
}

func (b *AnthropicBackend) Protocol() Protocol { return ProtocolAnthropic }

func (b *AnthropicBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	body := b.buildSimpleBody(req, false)
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := anthropicHTTPNew(ctx, AnthropicMessagesURL(b.baseURL), raw, b.auth)
	if err != nil {
		return "", err
	}
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", anthropicHTTPError(resp, bb)
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
