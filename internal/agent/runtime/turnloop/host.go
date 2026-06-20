package turnloop

import (
	"context"
	"encoding/json"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

type Host interface {
	WaitProviderReady(ctx context.Context) error
	EnsureCursorSidecar(ctx context.Context) error
	MachineMode() bool
	Out() io.Writer
	Config() *config.Root
	Provider() *config.Provider
	ModelName() string
	Backend() llm.CompletionBackend
	EphemeralSession() bool
	CompactionThreshold() int64
	MutateSession(fn func(*chatstore.Session))
	PersistSessionOrLog(context string)
	PersistSession() error
	SystemPrompt(disableThinking bool) (string, error)
	SystemPromptBtw(disableThinking bool) (string, error)
	BtwLinePrefixes() (userPrefix, assistantPrefix string)
	ReadBtwInput(out io.Writer, userPrefix, initial string) (string, error)
	LegacyToolsEnabled() bool
	LegacyToolsForced() bool
	ToolParams() ([]openai.ChatCompletionToolUnionParam, error)
	SessionMessagesSnapshot() ([]chatstore.Message, map[int]string)
	CITurn() int
	SetCITurn(int)
	CIEmit(cievents.Event)
	SetCIFinalContent(string)
	ToolCallsForCI([]chatstore.ToolCall) []map[string]any
	AllowedToolNames() (map[string]struct{}, error)
	NewLegacyStreamWriter(out io.Writer, enabled bool, allowed map[string]struct{}) (*tooling.LegacyStreamWriter, io.Writer)
	StreamOptsWithRetry(showThinking bool, reasonSink io.Writer) llm.StreamOpts
	StreamOptsCI(turnIdx int) llm.StreamOpts
	WrapLLMErr(err error) error
	ExternalToolBridge() bool
	StripCursorProxyInlineErrors(content string) (cleaned, fallback string)
	ResolveTurnInvocations(turn llm.AssistantTurnResult, legacySW *tooling.LegacyStreamWriter) ([]tooling.Invocation, []string, bool, error)
	HandleRejectedNativeToolCall() error
	HandleMalformedLegacyTool(err error) error
	SyncLegacyToolCallsToLastAssistant(invs []tooling.Invocation)
	SlashDeps(ctx context.Context) commands.Deps
	HandleProxyToolCorrection(msg string) error
	PrintToolInvocation(toolIdx int, name string, rawArgs json.RawMessage) int
	SetCurrentToolCpSeq(seq int)
	ExecTool(ctx context.Context, inv tooling.Invocation) (any, error)
	ApplyToolOutput(res any, toolName, toolCallID string) any
	NoteCIToolResult(res any)
	UserStopGeneration() error
	GenerationStoppedMessage() string
	ShowGenerationStopped(out io.Writer)
	IsMalformedLegacyToolErr(err error) bool
	BindTurnOut(w io.Writer) func()
}
