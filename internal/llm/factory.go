package llm

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func NewCompletionBackend(ctx context.Context, cfg *config.Root, p *config.Provider) (CompletionBackend, error) {
	if p == nil {
		return nil, fmt.Errorf("nil provider")
	}
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
		return NewAnthropicBackend(p.BaseURL, auth), nil
	default:
		client, err := newOpenAIClient(ctx, cfg, p)
		if err != nil {
			return nil, err
		}
		return &OpenAIBackend{Client: client}, nil
	}
}

func newOpenAIClient(ctx context.Context, cfg *config.Root, p *config.Provider) (openai.Client, error) {
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
	}
	if p.IsChatGPTSub() {
		opts = append(opts, codex.WithChatGPTSubMiddleware(p.OAuthAccountID))
	}
	return openai.NewClient(opts...), nil
}

func OpenAIClientFromBackend(b CompletionBackend) (openai.Client, bool) {
	ob, ok := b.(*OpenAIBackend)
	if !ok || ob == nil {
		return openai.Client{}, false
	}
	return ob.Client, true
}
