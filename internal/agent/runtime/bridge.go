package agentruntime

import (
	"context"
	"encoding/json"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/editor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

type turnHost struct{ *Runtime }

func (r *Runtime) runAgentTurns(ctx context.Context) error {
	return turnloop.Run(ctx, turnHost{r})
}

func (h turnHost) WaitProviderReady(ctx context.Context) error {
	return h.Runtime.waitProviderReady(ctx)
}
func (h turnHost) EnsureCursorSidecar(ctx context.Context) error {
	h.Runtime.ensureCursorSidecar(ctx)
	return nil
}
func (h turnHost) MachineMode() bool                         { return h.Runtime.machineMode() }
func (h turnHost) Out() io.Writer                            { return h.Runtime.Out }
func (h turnHost) Config() *config.Root                      { return h.Runtime.Cfg }
func (h turnHost) Provider() *config.Provider                { return h.Runtime.Prov }
func (h turnHost) ModelName() string                         { return h.Runtime.Model }
func (h turnHost) Backend() llm.CompletionBackend            { return h.Runtime.Backend }
func (h turnHost) EphemeralSession() bool                    { return h.Runtime.EphemeralSession }
func (h turnHost) CompactionThreshold() int64                { return h.Runtime.CompactionThresholdTokens }
func (h turnHost) MutateSession(fn func(*chatstore.Session)) { h.Runtime.mutateSession(fn) }
func (h turnHost) PersistSessionOrLog(context string)        { h.Runtime.persistSessionOrLog(context) }
func (h turnHost) PersistSession() error                     { return h.Runtime.persistSession() }
func (h turnHost) SystemPrompt(disableThinking bool) (string, error) {
	return h.Runtime.systemPrompt(disableThinking)
}
func (h turnHost) SystemPromptBtw(disableThinking bool) (string, error) {
	return h.Runtime.systemPromptBtw(disableThinking)
}
func (h turnHost) LegacyToolsEnabled() bool { return h.Runtime.legacyToolsEnabled() }
func (h turnHost) LegacyToolsForced() bool  { return h.Runtime.legacyToolsForced() }
func (h turnHost) ToolParams() ([]openai.ChatCompletionToolUnionParam, error) {
	return h.Runtime.toolParams()
}
func (h turnHost) SessionMessagesSnapshot() ([]chatstore.Message, map[int]string) {
	return h.Runtime.sessionMessagesSnapshot()
}
func (h turnHost) BtwLinePrefixes() (string, string) {
	h.Runtime.chatPersistMu.Lock()
	defer h.Runtime.chatPersistMu.Unlock()
	seq := h.Runtime.Session.CheckpointLast
	branch := h.Runtime.Session.CheckpointBranchSuffix
	return checkpoint.FormatLinePrefix(seq, branch), checkpoint.FormatLinePrefix(seq, branch)
}
func (h turnHost) ReadBtwInput(out io.Writer, userPrefix, initial string) (string, error) {
	return editor.ReadMultilineInitial(editor.Host{
		RL:                     h.Runtime.RL,
		Out:                    out,
		InputInterrupt:         commands.ReplStartupInterrupt(),
		CompleteEnv:            replcomplete.EnvFrom(h.Runtime),
		PromptPrimary:          func() string { return userPrefix + termcolor.WrapUserReadline("You: ") },
		PromptContinue:         func() string { return userPrefix + termcolor.WrapUserReadline(".... ") },
		ClipboardPasteForStdin: h.Runtime.replClipboardPasteTag,
	}, editor.NewHistory(), initial)
}
func (h turnHost) CITurn() int                { return h.Runtime.ciTurn }
func (h turnHost) SetCITurn(n int)            { h.Runtime.ciTurn = n }
func (h turnHost) CIEmit(ev cievents.Event)   { h.Runtime.ciEmit(ev) }
func (h turnHost) SetCIFinalContent(s string) { h.Runtime.ciFinalContent = s }
func (h turnHost) ToolCallsForCI(tcs []chatstore.ToolCall) []map[string]any {
	return toolCallsCI(tcs)
}
func (h turnHost) AllowedToolNames() (map[string]struct{}, error) {
	return h.Runtime.allowedToolNames()
}
func (h turnHost) NewLegacyStreamWriter(out io.Writer, enabled bool, allowed map[string]struct{}) (*tooling.LegacyStreamWriter, io.Writer) {
	return newLegacyStreamWriter(out, enabled, allowed)
}
func (h turnHost) StreamOptsWithRetry(showThinking bool, reasonSink io.Writer) llm.StreamOpts {
	return h.Runtime.streamOptsWithRetry(showThinking, reasonSink)
}
func (h turnHost) StreamOptsCI(turnIdx int) llm.StreamOpts { return h.Runtime.streamOptsCI(turnIdx) }
func (h turnHost) WrapLLMErr(err error) error              { return h.Runtime.wrapLLMErr(err) }
func (h turnHost) ExternalToolBridge() bool                { return h.Runtime.externalToolBridge() }
func (h turnHost) StripCursorProxyInlineErrors(content string) (string, string) {
	return stripCursorProxyInlineErrors(content)
}
func (h turnHost) ResolveTurnInvocations(turn llm.AssistantTurnResult, legacySW *tooling.LegacyStreamWriter) ([]tooling.Invocation, []string, bool, error) {
	return h.Runtime.ResolveTurnInvocations(turn, legacySW)
}
func (h turnHost) HandleRejectedNativeToolCall() error {
	return h.Runtime.handleRejectedNativeToolCall()
}
func (h turnHost) HandleMalformedLegacyTool(err error) error {
	return h.Runtime.handleMalformedLegacyTool(err)
}
func (h turnHost) SyncLegacyToolCallsToLastAssistant(invs []tooling.Invocation) {
	h.Runtime.syncLegacyToolCallsToLastAssistant(invs)
}
func (h turnHost) SlashDeps(ctx context.Context) commands.Deps { return h.Runtime.slashDeps(ctx) }
func (h turnHost) HandleProxyToolCorrection(msg string) error {
	return h.Runtime.handleProxyToolCorrection(msg)
}
func (h turnHost) PrintToolInvocation(toolIdx int, name string, rawArgs json.RawMessage) int {
	return h.Runtime.printToolInvocation(toolIdx, name, rawArgs)
}
func (h turnHost) SetCurrentToolCpSeq(seq int) { h.Runtime.currentToolCpSeq = seq }
func (h turnHost) ExecTool(ctx context.Context, inv tooling.Invocation) (any, error) {
	return h.Runtime.execTool(ctx, inv)
}
func (h turnHost) ApplyToolOutput(res any, toolName, toolCallID string) any {
	return h.Runtime.applyToolOutput(res, toolName, toolCallID)
}
func (h turnHost) NoteCIToolResult(res any)                { h.Runtime.noteCIToolResult(res) }
func (h turnHost) UserStopGeneration() error               { return errUserStopGeneration }
func (h turnHost) GenerationStoppedMessage() string        { return cliMsgGenerationStopped }
func (h turnHost) ShowGenerationStopped(out io.Writer)     { showGenerationStopped(out) }
func (h turnHost) IsMalformedLegacyToolErr(err error) bool { return isMalformedLegacyToolErr(err) }
func (h turnHost) BindTurnOut(w io.Writer) func() {
	prev := h.Runtime.Out
	h.Runtime.Out = w
	return func() { h.Runtime.Out = prev }
}
