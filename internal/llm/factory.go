package llm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func NewCompletionBackend(ctx context.Context, cfg *config.Root, p *config.Provider) (CompletionBackend, error) {
	if p == nil {
		return nil, fmt.Errorf("nil provider")
	}
	policy := config.EffectiveAPIResilience(cfg)
	hostKey := HostKeyFromBaseURL(p.BaseURL)
	httpClient := NewResilientHTTPClient(policy)
	var inner CompletionBackend
	switch p.EffectiveAPIProtocol() {
	case config.APIProtocolAnthropic:
		bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
		if err != nil {
			return nil, err
		}
		auth := AnthropicAuthFromAPIKey(bearer)
		if p.UsesAnthropicOAuthBearer() {
			auth = AnthropicAuthFromOAuthBearer(bearer)
		}
		inner = NewAnthropicBackendWithClient(p.BaseURL, auth, httpClient)
	default:
		client, err := newOpenAIClient(ctx, cfg, p, httpClient)
		if err != nil {
			return nil, err
		}
		inner = &OpenAIBackend{Client: client}
	}
	return NewResilientBackend(inner, hostKey, policy, defaultCircuits), nil
}

func newOpenAIClient(ctx context.Context, cfg *config.Root, p *config.Provider, httpClient *http.Client) (openai.Client, error) {
	config.EnsureChatGPTSubBaseURL(p)
	if p.IsChatGPTSub() && cfg != nil {
		if ep := config.ProviderByName(cfg, p.Name); ep != nil {
			ep.BaseURL = p.BaseURL
			_ = config.Save(cfg)
		}
	}
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return openai.Client{}, err
	}
	opts := []option.RequestOption{
		option.WithAPIKey(bearer),
		option.WithBaseURL(p.BaseURL),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(0),
	}
	if p.IsChatGPTSub() {
		opts = append(opts, codex.WithChatGPTSubMiddleware(p.OAuthAccountID))
	}
	return openai.NewClient(opts...), nil
}

func OpenAIClientFromBackend(b CompletionBackend) (openai.Client, bool) {
	if rb, ok := b.(*ResilientBackend); ok && rb != nil {
		return OpenAIClientFromBackend(rb.Inner)
	}
	ob, ok := b.(*OpenAIBackend)
	if !ok || ob == nil {
		return openai.Client{}, false
	}
	return ob.Client, true
}
