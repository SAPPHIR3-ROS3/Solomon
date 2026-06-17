package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
)

type backendLLM struct {
	ctx     context.Context
	backend llm.CompletionBackend
	cfg     *config.Root
	model   string
	usage   *apitype.UsageStats
}

func NewBackendLLM(ctx context.Context, backend llm.CompletionBackend, cfg *config.Root, model string, usage *apitype.UsageStats) LLMCaller {
	return &backendLLM{ctx: ctx, backend: backend, cfg: cfg, model: model, usage: usage}
}

func (b *backendLLM) Complete(userPrompt string, maxTokens int) (string, apitype.UsageStats, error) {
	if b.backend == nil {
		return "", apitype.UsageStats{}, fmt.Errorf("llm backend unavailable")
	}
	text, err := b.backend.CompleteText(b.ctx, llm.SimpleCompletionRequest{
		Cfg:                   b.cfg,
		Model:                 b.model,
		System:                "You are a research assistant. Follow instructions precisely.",
		User:                  userPrompt,
		ForceDisableReasoning: true,
	})
	if err != nil {
		return "", apitype.UsageStats{}, err
	}
	text = strings.TrimSpace(text)
	return text, apitype.UsageStats{}, nil
}

func mergeUsage(dst *apitype.UsageStats, src apitype.UsageStats) {
	dst.PromptTokens += src.PromptTokens
	dst.ResponseTokens += src.ResponseTokens
	dst.TotalTokens += src.TotalTokens
	dst.ReasoningTokens += src.ReasoningTokens
	dst.CachedPromptTokens += src.CachedPromptTokens
}
