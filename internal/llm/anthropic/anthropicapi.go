package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Backend struct {
	baseURL       string
	auth          Auth
	httpClient    *http.Client
	tokenProvider func(context.Context) (string, error)
}

// WithTokenProvider renews subscription credentials before each request, including
// requests made by a long-lived runtime. Each request uses its own auth snapshot.
func (b *Backend) WithTokenProvider(provider func(context.Context) (string, error)) *Backend {
	b.tokenProvider = provider
	return b
}

func (b *Backend) forRequest(ctx context.Context) (*Backend, error) {
	if b.tokenProvider == nil {
		return b, nil
	}
	token, err := b.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}
	copy := *b
	copy.auth.Token = token
	copy.tokenProvider = nil
	return &copy, nil
}

func NewBackend(baseURL string, auth Auth) *Backend {
	return NewBackendWithClient(baseURL, auth, nil)
}

func NewBackendWithClient(baseURL string, auth Auth, client *http.Client) *Backend {
	if client == nil {
		client = &http.Client{}
	}
	return &Backend{baseURL: strings.TrimSpace(baseURL), auth: auth, httpClient: client}
}

func (b *Backend) Protocol() apitype.Protocol { return apitype.ProtocolAnthropic }

// sdkParams keeps Solomon's message/image/tool mapping while letting the SDK
// validate and serialize the Anthropic parameter unions.
func sdkParams(body map[string]any) (sdk.MessageNewParams, error) {
	if system, ok := body["system"].(string); ok {
		body["system"] = []contentBlock{{"type": "text", "text": system}}
	}
	delete(body, "stream")
	raw, err := json.Marshal(body)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}
	var params sdk.MessageNewParams
	err = json.Unmarshal(raw, &params)
	return params, err
}

func (b *Backend) client() sdk.Client {
	return claude.NewClient(b.baseURL, b.auth.Token, b.auth.OAuth(), b.httpClient)
}

func (b *Backend) CompleteText(ctx context.Context, req apitype.SimpleCompletionRequest) (string, error) {
	current, err := b.forRequest(ctx)
	if err != nil {
		return "", err
	}
	b = current
	params, err := sdkParams(b.buildSimpleBody(req, false))
	if err != nil {
		return "", err
	}
	client := b.client()
	opts := []option.RequestOption{option.WithJSONSet("stream", false)}
	if b.auth.OAuth() {
		opts = append(opts, option.WithQuery("beta", "true"), option.WithHeader("anthropic-beta", OAuthBeta))
	}
	msg, err := client.Messages.New(ctx, params, opts...)
	if err != nil {
		return "", claude.SDKError(err)
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
	current, err := b.forRequest(ctx)
	if err != nil {
		return nil, err
	}
	b = current
	oauth := b.auth.Kind == AuthOAuthBearer
	ids, err := modelsapi.ListAnthropicWithClient(ctx, b.baseURL, b.auth.Token, oauth, b.httpClient)
	if err != nil {
		return nil, err
	}
	return modelsapi.PickAnthropicFlagshipModels(ids), nil
}
