package llm

import (
	"context"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

type Protocol string

const (
	ProtocolOpenAI     Protocol = "openai"
	ProtocolAnthropic  Protocol = "anthropic"
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
}

type SimpleCompletionRequest struct {
	Cfg                   *config.Root
	Model                 string
	System                string
	User                  string
	ForceDisableReasoning bool
}

type CompletionBackend interface {
	Protocol() Protocol
	StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error)
	StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error)
	CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error)
	ListModels(ctx context.Context) ([]string, error)
}
