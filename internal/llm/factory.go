package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/anthropic"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func NewCompletionBackend(ctx context.Context, cfg *config.Root, p *config.Provider) (CompletionBackend, error) {
	if p == nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "completion backend nil provider", logging.LogOptions{Params: nil})
		return nil, fmt.Errorf("nil provider")
	}
	config.EnsureClaudeSubBaseURL(p)
	config.EnsureCursorSubBaseURL(p)
	policy := config.EffectiveAPIResilience(cfg)
	if p.IsCursorSub() {
		// A streamed tool call can have side effects; never replay the run.
		policy.MaxRetries = 1
		policy.ReadTimeout = 0
	}
	hostKey := HostKeyFromBaseURL(p.BaseURL)
	httpClient := NewResilientHTTPClient(policy)
	var inner CompletionBackend
	switch {
	case p.IsCursorSub():
		inner = &cursorSubBackend{cfg: cfg, provider: p, httpClient: httpClient}
	case p.EffectiveAPIProtocol() == config.APIProtocolAnthropic:
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
	if p.IsCursorAPI() && !p.IsCursorSub() {
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

type cursorSubBackend struct {
	cfg        *config.Root
	provider   *config.Provider
	httpClient *http.Client
	mu         sync.Mutex
	pending    *cursorPendingTool
}

func (b *cursorSubBackend) Protocol() Protocol { return ProtocolOpenAI }

func (b *cursorSubBackend) StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	effort := req.ReasoningEffort
	if req.ForceDisableReasoning {
		effort = "none"
	}
	if b.cfg != nil && strings.TrimSpace(effort) == "" {
		effort = b.cfg.ReasoningEffortLabel()
	}
	result, err := b.streamAgent(ctx, req.Model, req.System, req.Messages, req.ImageFiles, req.Tools, contentOut, effort, opts)
	if err != nil {
		return AssistantTurnResult{}, err
	}
	out := AssistantTurnResult{Content: result.Content, FinishReason: result.FinishReason}
	if out.FinishReason == "" {
		out.FinishReason = FinishReasonStop
	}
	for _, call := range result.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, AssistantToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	out.Usage = UsageStats{
		PromptTokens:              result.Usage.PromptTokens,
		CachedPromptTokens:        result.Usage.CachedPromptTokens,
		CacheCreationPromptTokens: result.Usage.CacheWriteTokens,
		ReasoningTokens:           result.Usage.ReasoningTokens,
		ResponseTokens:            result.Usage.CompletionTokens,
		TotalTokens:               result.Usage.TotalTokens,
	}
	return out, nil
}

func (b *cursorSubBackend) StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	result, err := b.streamAgent(ctx, req.Model, req.System, []chatstore.Message{{Role: "user", Content: req.User}}, nil, nil, contentOut, "", opts)
	return result.Content, UsageStats{PromptTokens: result.Usage.PromptTokens, CachedPromptTokens: result.Usage.CachedPromptTokens, CacheCreationPromptTokens: result.Usage.CacheWriteTokens, ReasoningTokens: result.Usage.ReasoningTokens, ResponseTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens}, err
}

func (b *cursorSubBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	result, err := b.streamAgent(ctx, req.Model, req.System, []chatstore.Message{{Role: "user", Content: req.User}}, nil, nil, io.Discard, "", StreamOpts{})
	return result.Content, err
}

func (b *cursorSubBackend) ListModels(ctx context.Context) ([]string, error) {
	if b == nil || b.provider == nil {
		return nil, fmt.Errorf("Cursor Sub backend missing provider")
	}
	session, err := config.ResolveCursorSessionBearer(ctx, b.cfg, b.provider)
	if err != nil {
		return nil, err
	}
	return cursorauth.ListAvailableModels(ctx, session)
}
