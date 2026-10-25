package apitype

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
)

// Finish reasons normalized across completion providers.
const (
	FinishReasonStop          = "stop"
	FinishReasonToolCalls     = "tool_calls"
	FinishReasonLength        = "length"
	FinishReasonContentFilter = "content_filter"
)

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

type TurnRequest struct {
	Cfg                   *config.Root
	Model                 string
	System                string
	Messages              []chatstore.Message
	ImageFiles            map[int]string
	Tools                 []ToolDef
	ParallelToolCalls     bool
	ForceDisableReasoning bool
	ReasoningEffort       string
	CacheKey              string
}

type SimpleCompletionRequest struct {
	Cfg                   *config.Root
	Model                 string
	System                string
	User                  string
	ForceDisableReasoning bool
	CacheKey              string
}

type AssistantToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Cache counters use -1 when the provider does not report a value. Zero is explicit.
type UsageStats struct {
	PromptTokens              int64
	CachedPromptTokens        int64
	CacheCreationPromptTokens int64
	ReasoningTokens           int64
	ResponseTokens            int64
	TotalTokens               int64
	OutputTPS                 float64
	TTFTSecs                  float64
	PromptTPS                 float64
	TurnWallSecs              float64
	ThoughtSecs               float64
}

type AssistantTurnResult struct {
	Content            string
	ReasoningText      string
	ReasoningEncrypted string
	ReasoningItem      string
	// Opaque signed provider blocks must survive tool continuations unchanged.
	AnthropicThinking []json.RawMessage
	ToolCalls         []AssistantToolCall
	// FinishReason is the provider's normalized reason for ending this
	// assistant generation. It is empty when the provider does not report one.
	FinishReason        string
	Usage               UsageStats
	ProxyToolCorrection string
}

type StreamOpts struct {
	ShowThinking  bool
	ReasoningSink io.Writer
	OnDelta       func(channel, text string)
	OnRetry       func(attempt int, max int, err error, wait time.Duration)
}

type CompletionBackend interface {
	Protocol() Protocol
	StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error)
	StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error)
	CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error)
	ListModels(ctx context.Context) ([]string, error)
}
