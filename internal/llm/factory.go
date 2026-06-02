package llm

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/anthropic"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func NewCompletionBackend(ctx context.Context, cfg *config.Root, p *config.Provider) (CompletionBackend, error) {
	if p == nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "completion backend nil provider", logging.LogOptions{Params: nil})
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
			logging.Log(logging.ERROR_LOG_LEVEL, "completion backend resolve bearer failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
			return nil, err
		}
		auth := anthropic.AuthFromAPIKey(bearer)
		if p.UsesAnthropicOAuthBearer() {
			auth = anthropic.AuthFromOAuthBearer(bearer)
		}
		inner = anthropic.NewBackendWithClient(p.BaseURL, auth, httpClient)
	default:
		client, err := newOpenAIClient(ctx, cfg, p, httpClient)
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "completion backend openai client failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
			return nil, err
		}
		inner = &OpenAIBackend{Client: client}
	}
	rb := NewResilientBackend(inner, hostKey, policy, defaultCircuits)
	if p.IsCursorAPI() {
		cfgCopy := cfg
		rb.SidecarRevive = func(ctx context.Context, err error) {
			cwd, _ := os.Getwd()
			cursorint.ReviveSidecarIfConfigured(ctx, cfgCopy, cwd, err)
		}
	}
	return rb, nil
}

func newOpenAIClient(ctx context.Context, cfg *config.Root, p *config.Provider, httpClient *http.Client) (openai.Client, error) {
	config.EnsureChatGPTSubBaseURL(p)
	if p.IsChatGPTSub() && cfg != nil {
		if ep := config.ProviderByName(cfg, p.Name); ep != nil {
			ep.BaseURL = p.BaseURL
			if err := config.Save(cfg); err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "save config after ChatGPT Sub base URL update failed", logging.LogOptions{Params: map[string]any{"err": err.Error(), "provider": p.Name}})
			}
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
