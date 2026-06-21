package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

type Backend struct {
	baseURL    string
	auth       Auth
	httpClient *http.Client
}

func NewBackend(baseURL string, auth Auth) *Backend {
	return NewBackendWithClient(baseURL, auth, nil)
}

func NewBackendWithClient(baseURL string, auth Auth, client *http.Client) *Backend {
	if client == nil {
		client = httpDefault()
	}
	return &Backend{baseURL: strings.TrimSpace(baseURL), auth: auth, httpClient: client}
}

func (b *Backend) Protocol() apitype.Protocol { return apitype.ProtocolAnthropic }

func (b *Backend) CompleteText(ctx context.Context, req apitype.SimpleCompletionRequest) (string, error) {
	body := b.buildSimpleBody(req, false)
	raw, err := json.Marshal(body)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "anthropic complete marshal failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	httpReq, err := httpNew(ctx, MessagesURL(b.baseURL), raw, b.auth)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "anthropic complete request build failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", httpError(resp, bb)
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

func (b *Backend) ListModels(ctx context.Context) ([]string, error) {
	_ = ctx
	oauth := b.auth.Kind == AuthOAuthBearer
	ids, err := modelsapi.ListAnthropic(b.baseURL, b.auth.Token, oauth)
	if err != nil {
		ids = modelsapi.CuratedAnthropicModels()
	}
	return modelsapi.PickAnthropicFlagshipModels(ids), nil
}
