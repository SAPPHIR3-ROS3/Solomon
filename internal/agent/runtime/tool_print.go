package agentruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const legacyToolJSONCorrectionUserMsg = "Your previous reply contained a malformed tool-invocation block. Preferred shape:\n<tool_calls>\n<tool name=\"TOOL_NAME\">\n<intent>brief purpose</intent>\n<args>{\"key\":\"value\"}</args>\n</tool>\n</tool_calls>\nAlso accepted: <tool_call>{\"name\":\"TOOL_NAME\",\"arguments\":{...}}</tool_call> or <functioncall>{\"name\":\"TOOL_NAME\",\"arguments\":{...}}</functioncall> with valid JSON. Close <tool name=\"...\"> with </tool>, not </tool_call>. Send a corrected block only, or continue without tools if you meant plain text."

const nativeBridgeToolCorrectionUserMsg = "Your previous reply did not include valid native API tool_calls. Emit native Solomon tools only (orchestrate, searchTools, subagent, switchMode, searchSkill, loadSkill) via API tool_calls with JSON arguments that match each tool schema — not <tool_calls> XML or plain-text tool narration. For workspace read/edit/shell/find/MCP work, call searchTools if unsure, then orchestrate (package main, import \"sdk\") — never emit deferred tools (readFile, editFile, shell, find, …) as direct native tool_calls. Send a corrected invocation or continue without tools if you meant plain text."

const cursorProxyOrchestrateFooter = "Cursor built-ins are disabled. Use native tool_calls only: searchTools (discover deferred SDK signatures), orchestrate (run workspace scripts), searchSkill and loadSkill (skills)."

const cursorProxyInlineErrorPrefix = "[error] Cursor internal tool call blocked by Solomon proxy:"

const blockedMcpExternalLabel = "mcp:external"

var cursorNativeAliases = map[string]string{
	"read": "readFile", "Read": "readFile", "read_file": "readFile", "ReadFile": "readFile", "readfile": "readFile",
	"shell": "shell", "Shell": "shell", "bash": "shell", "Bash": "shell", "run_terminal_cmd": "shell", "terminal": "shell",
	"edit": "editFile", "Edit": "editFile", "write": "editFile", "Write": "editFile", "StrReplace": "editFile", "strReplace": "editFile", "str_replace": "editFile", "search_replace": "editFile", "Delete": "editFile", "delete": "editFile",
	"find": "find", "Find": "find", "Grep": "find", "grep": "find", "Glob": "find", "glob": "find", "ListDir": "find", "list_dir": "find", "listDir": "find", "ls": "find", "ripgrep": "find", "rg": "find", "SemanticSearch": "find", "semanticSearch": "find", "semantic_search": "find",
	"Task": "subagent", "task": "subagent",
	"WebFetch": "fetchWeb", "webFetch": "fetchWeb", "web_fetch": "fetchWeb", "Fetch": "fetchWeb", "fetch": "fetchWeb",
	"WebSearch": "webSearch", "webSearch": "webSearch", "web_search": "webSearch",
}

var cursorHardDenyTools = map[string]struct{}{
	"AskQuestion": {}, "ask_question": {}, "askQuestion": {},
	"GenerateImage": {}, "generate_image": {}, "generateImage": {},
	"Await": {}, "await": {},
	"ApplyPatch": {}, "apply_patch": {}, "applyPatch": {},
}

var cursorRedirectExtra = map[string]struct{}{
	"ReadLints": {}, "read_lints": {}, "readLints": {},
	"EditNotebook": {}, "edit_notebook": {}, "editNotebook": {},
	"TodoWrite": {}, "todo_write": {}, "todoWrite": {},
	"CallMcpTool": {}, "call_mcp_tool": {}, "callMcpTool": {},
	"FetchMcpResource": {}, "fetch_mcp_resource": {}, "fetchMcpResource": {},
	"ListMcpResources": {}, "list_mcp_resources": {}, "listMcpResources": {},
}

var deferredSolomonToolNames = map[string]struct{}{
	"readFile": {}, "shell": {}, "editFile": {}, "find": {}, "listDir": {},
	"fetchWeb": {}, "webSearch": {}, "createPlan": {}, "editPlan": {}, "buildPlan": {},
}

func isBrowserCursorTool(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "browser_") {
		return true
	}
	if len(trimmed) >= 8 {
		if strings.HasPrefix(trimmed, "browser") || strings.HasPrefix(trimmed, "Browser") {
			c := trimmed[7]
			if c >= 'A' && c <= 'Z' {
				return true
			}
		}
	}
	return false
}

func shouldHardDenyCursorTool(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if _, ok := cursorHardDenyTools[trimmed]; ok {
		return true
	}
	return isBrowserCursorTool(trimmed)
}

func shouldRedirectCursorTool(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if _, ok := cursorRedirectExtra[trimmed]; ok {
		return true
	}
	_, ok := cursorNativeAliases[trimmed]
	return ok
}

func isHardDenyBlockedCursorLabel(label string) bool {
	trimmed := strings.TrimSpace(label)
	if trimmed == blockedMcpExternalLabel {
		return true
	}
	return shouldHardDenyCursorTool(trimmed)
}

func shouldBlockDeferredSolomonTool(name string) bool {
	_, ok := deferredSolomonToolNames[strings.TrimSpace(name)]
	return ok
}

func cursorToolRedirectTarget(cursorName string) string {
	return cursorNativeAliases[strings.TrimSpace(cursorName)]
}

func hardDenyCorrectionHint(toolName string) string {
	trimmed := strings.TrimSpace(toolName)
	if trimmed == "" {
		return ""
	}
	if trimmed == blockedMcpExternalLabel {
		return "External MCP servers (including Cursor IDE browser) are not available on this host."
	}
	if isBrowserCursorTool(trimmed) {
		return "Cursor IDE browser tools are not available on this host."
	}
	key := strings.ToLower(strings.ReplaceAll(trimmed, "_", ""))
	switch key {
	case "askquestion":
		return "Ask the user in plain text instead of AskQuestion."
	case "generateimage":
		return "Describe the image in text or use an orchestrate workaround; image generation is not available."
	case "await":
		return "Use synchronous orchestrate or subagent async polling instead of Await."
	case "applypatch":
		return "Use orchestrate with the sandbox write/replace SDK for edits; unified diff ApplyPatch is not supported."
	default:
		if shouldHardDenyCursorTool(trimmed) {
			return "This Cursor tool is not available on this host."
		}
		return ""
	}
}

func redirectExtraCorrectionHint(toolName string) string {
	key := strings.ToLower(strings.ReplaceAll(toolName, "_", ""))
	switch key {
	case "readlints":
		return "Lint diagnostics: use orchestrate; Cursor ReadLints is not available on this host."
	case "editnotebook":
		return "Notebook edits: use orchestrate until a dedicated notebook tool ships."
	case "todowrite":
		return "Plan todos: use orchestrate with addTodo, todoList, checkTodo, or related plan SDK helpers."
	case "callmcptool", "fetchmcpresource", "listmcpresources":
		return "MCP work: call searchTools for schemas, then orchestrate with the MCP sandbox SDK."
	default:
		return ""
	}
}

func redirectCorrectionHint(toolName string) string {
	trimmed := strings.TrimSpace(toolName)
	if trimmed == "" || isHardDenyBlockedCursorLabel(trimmed) {
		return ""
	}
	if strings.HasPrefix(trimmed, "mcp:") {
		deferred := trimmed[4:]
		if shouldBlockDeferredSolomonTool(deferred) {
			return deferred + ": call searchTools, then orchestrate with the matching sandbox SDK — not a direct native tool_call."
		}
		return ""
	}
	if extra := redirectExtraCorrectionHint(trimmed); extra != "" {
		return extra
	}
	if !shouldRedirectCursorTool(trimmed) {
		return ""
	}
	switch cursorToolRedirectTarget(trimmed) {
	case "readFile":
		return "Cursor Read is disabled. Call searchTools, then orchestrate with sdk.ReadFile."
	case "editFile":
		return "Cursor edits are disabled. Call searchTools, then orchestrate with sdk.WriteFile, sdk.ReplaceInFile, or sdk.DeleteFile."
	case "shell":
		return "Cursor Shell is disabled. Call searchTools, then orchestrate with sdk.Shell (sync only)."
	case "find":
		return "Cursor Grep/Glob are disabled. Call searchTools, then orchestrate with sdk.Glob, sdk.Grep, or sdk.GrepLines."
	case "subagent":
		return "Nested agent work: emit native subagent via <tool_calls> or tool_calls."
	case "fetchWeb":
		return "HTTP fetch: orchestrate with sdk.FetchWeb."
	case "webSearch":
		return "Web search: orchestrate with sdk.WebSearch."
	default:
		return "Call searchTools, then orchestrate with the sandbox SDK."
	}
}

func correctionHintForBlockedCursorTool(toolName string) string {
	if hint := hardDenyCorrectionHint(toolName); hint != "" {
		return hint
	}
	return redirectCorrectionHint(toolName)
}

func uniqueNonEmptyTrimmed(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func cursorProxyToolCorrectionMessage(blocked []string) string {
	unique := uniqueNonEmptyTrimmed(blocked)
	if len(unique) == 0 {
		return ""
	}
	parts := []string{"Blocked by Solomon proxy: " + strings.Join(unique, ", ") + "."}
	var hints []string
	for _, name := range unique {
		if hint := correctionHintForBlockedCursorTool(name); hint != "" {
			hints = append(hints, hint)
		}
	}
	if len(hints) > 0 {
		parts = append(parts, strings.Join(hints, " "))
	}
	includeFooter := false
	for _, n := range unique {
		if !isHardDenyBlockedCursorLabel(n) && (shouldRedirectCursorTool(n) || strings.HasPrefix(n, "mcp:")) {
			includeFooter = true
			break
		}
	}
	if includeFooter {
		parts = append(parts, cursorProxyOrchestrateFooter)
	}
	parts = append(parts, "Reply with a corrected invocation or plain text.")
	return strings.Join(parts, " ")
}

func newLegacyStreamWriter(out io.Writer, enabled bool, allowed map[string]struct{}) (*tooling.LegacyStreamWriter, io.Writer) {
	if !enabled {
		return nil, out
	}
	lsw := tooling.NewLegacyStreamWriter(out, nil, allowed)
	return lsw, lsw
}

func (r *Runtime) stampAssistantToolCallCheckpoint(toolIdx, cpSeq int, branchKey string) {
	r.mutateSession(func(s *chatstore.Session) {
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role != "assistant" {
				continue
			}
			if toolIdx >= len(s.Messages[i].ToolCalls) {
				return
			}
			tc := &s.Messages[i].ToolCalls[toolIdx]
			tc.CheckpointSeq = cpSeq
			tc.CpSeqSet = true
			tc.CheckpointBranchKey = branchKey
			return
		}
	})
}

func (r *Runtime) printToolInvocation(toolIdx int, name string, rawArgs json.RawMessage) int {
	var cpSeq int
	var branchKey string
	r.mutateSession(func(s *chatstore.Session) {
		cpSeq = checkpoint.Bump(s)
		branchKey = s.CheckpointBranchSuffix
	})
	r.stampAssistantToolCallCheckpoint(toolIdx, cpSeq, branchKey)
	if intent := tooling.ExtractToolIntent(rawArgs); intent != "" {
		tooling.WriteToolDisplayLines(r.Out, cpSeq, branchKey, []string{termcolor.WrapThinking(intent)})
	}
	tooling.WriteToolDisplayLines(r.Out, cpSeq, branchKey, formatToolDisplayLines(name, rawArgs))
	return cpSeq
}

const legacyNativeToolRejectedUserMsg = "Native API tool_calls are disabled because legacy tools force is ON. Do not use function calling. Emit tool invocations only inside a <tool_calls> XML block as described in the system prompt."

func (r *Runtime) handleRejectedNativeToolCall() error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, "Legacy tools force: native API tool_calls were ignored. Use <tool_calls> XML in assistant text.")
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: legacyNativeToolRejectedUserMsg}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func (r *Runtime) toolInvocationCorrectionUserMsg() string {
	if r != nil && r.externalToolBridge() && !r.legacyToolsForced() {
		return nativeBridgeToolCorrectionUserMsg
	}
	return legacyToolJSONCorrectionUserMsg
}

func (r *Runtime) handleMalformedLegacyTool(err error) error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, legacyToolScreenMessage(err))
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	correction := legacyToolJSONCorrectionUserMsg
	if r != nil {
		correction = r.toolInvocationCorrectionUserMsg()
	}
	return r.injectToolCorrectionUserMsg(correction)
}

func (r *Runtime) handleProxyToolCorrection(msg string) error {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, "Cursor proxy rejected a built-in tool call; retry with Solomon native tools: searchTools (discover deferred SDK), orchestrate (run workspace scripts), searchSkill, loadSkill.")
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	return r.injectToolCorrectionUserMsg(msg)
}

func (r *Runtime) injectToolCorrectionUserMsg(correction string) error {
	correction = strings.TrimSpace(correction)
	if correction == "" {
		return nil
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: correction}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func stripCursorProxyInlineErrors(content string) (string, string) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	var blocked []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, cursorProxyInlineErrorPrefix) {
			name := strings.TrimSpace(strings.TrimPrefix(trim, cursorProxyInlineErrorPrefix))
			if name != "" {
				blocked = append(blocked, name)
			}
			continue
		}
		out = append(out, line)
	}
	cleaned := strings.TrimSpace(strings.Join(out, "\n"))
	if len(blocked) == 0 {
		return content, ""
	}
	fallback := cursorProxyToolCorrectionMessage(blocked)
	return cleaned, fallback
}

func CursorProxyToolCorrectionMessageForTest(blocked []string) string {
	return cursorProxyToolCorrectionMessage(blocked)
}

func StripCursorProxyInlineErrorsForTest(content string) (string, string) {
	return stripCursorProxyInlineErrors(content)
}

func NativeBridgeToolCorrectionUserMsgForTest() string {
	return nativeBridgeToolCorrectionUserMsg
}

func legacyToolScreenMessage(err error) string {
	return tooling.UserFacingLegacyToolError(err)
}

func isMalformedLegacyToolErr(err error) bool {
	return errors.Is(err, tooling.ErrMalformedLegacyTool) || errors.Is(err, tooling.ErrUnknownLegacyTool)
}

func formatToolDisplayLines(name string, rawArgs json.RawMessage) []string {
	return tooling.FormatToolDisplayLines(name, rawArgs)
}

func formatToolPlainLines(name string, rawArgs json.RawMessage) []string {
	colored := tooling.FormatToolDisplayLines(name, rawArgs)
	out := make([]string, len(colored))
	for i, line := range colored {
		out[i] = stripANSI(line)
	}
	return out
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
