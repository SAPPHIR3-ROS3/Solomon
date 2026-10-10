package agentruntime

import (
	"sort"
	"strings"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
)

func (r *Runtime) chatCorrectionMode() bool {
	return r != nil && agenttools.NormalizeMode(r.Mode) == "chat"
}

func (r *Runtime) correctionToolNames() map[string]struct{} {
	names, err := r.allowedToolNames()
	if err != nil {
		return map[string]struct{}{}
	}
	return names
}

func nativeChatCorrectionMessage(names map[string]struct{}) string {
	available := make([]string, 0, len(names))
	for name := range names {
		available = append(available, name)
	}
	sort.Strings(available)
	surface := "No native tools are available in this request; continue in plain text."
	if len(available) > 0 {
		surface = "Use native API tool_calls only from this request: " + strings.Join(available, ", ") + ". Match each declared JSON schema and include a non-empty intent string."
	}
	recovery := "Workspace implementation is unavailable in CHAT mode."
	if _, ok := names["switchMode"]; ok {
		recovery += " Call native switchMode before implementation."
	}
	return "This is CHAT mode. " + surface + " " + recovery + " Do not write <tool_calls> XML or plain-text tool narration; continue in plain text if no tool is needed."
}

func (r *Runtime) nativeBridgeCorrectionMessage() string {
	if r.chatCorrectionMode() {
		return nativeChatCorrectionMessage(r.correctionToolNames())
	}
	return nativeBridgeToolCorrectionUserMsg
}

func chatProxyToolCorrectionMessage(blocked []string, names map[string]struct{}) string {
	unique := uniqueNonEmptyTrimmed(blocked)
	if len(unique) == 0 {
		return ""
	}
	parts := []string{"Blocked by Solomon proxy: " + strings.Join(unique, ", ") + "."}
	switchHint := "Workspace implementation is unavailable in this request; continue in plain text."
	if _, ok := names["switchMode"]; ok {
		switchHint = "Call native switchMode before workspace implementation."
	}
	for _, name := range unique {
		key := strings.ToLower(strings.ReplaceAll(name, "_", ""))
		switch {
		case strings.HasSuffix(name, ":missing_intent"):
			parts = append(parts, "Every Solomon tool call requires a non-empty intent string; include intent in the arguments.")
		case key == "generateimage":
			parts = append(parts, "Image generation is unavailable; describe the image in plain text.")
		case key == "await":
			parts = append(parts, "Background workspace tasks are unavailable in CHAT mode. "+switchHint)
		case key == "applypatch":
			parts = append(parts, "Unified diff ApplyPatch is unsupported. "+switchHint)
		case isHardDenyBlockedCursorLabel(name):
			parts = append(parts, hardDenyCorrectionHint(name))
		case strings.HasPrefix(name, "mcp:"):
			parts = append(parts, "MCP actions are not exposed in CHAT mode; do not claim a result without a host tool result. "+switchHint)
		default:
			target := cursorToolRedirectTarget(name)
			if target == "fetchWeb" || target == "webSearch" {
				if _, ok := names[target]; ok {
					parts = append(parts, "Use the native "+target+" tool in CHAT mode.")
				} else {
					parts = append(parts, "The requested web capability is not exposed in this request; continue in plain text.")
				}
			} else if shouldRedirectCursorTool(name) || shouldBlockDeferredSolomonTool(name) {
				parts = append(parts, "This workspace or agent tool is unavailable in CHAT mode. "+switchHint)
			}
		}
	}
	parts = append(parts, nativeChatCorrectionMessage(names), "Reply with a corrected invocation or plain text.")
	return strings.Join(parts, " ")
}

func (r *Runtime) proxyToolCorrectionMessage(blocked []string) string {
	if r.chatCorrectionMode() {
		return chatProxyToolCorrectionMessage(blocked, r.correctionToolNames())
	}
	return cursorProxyToolCorrectionMessage(blocked)
}

func (r *Runtime) proxyCorrectionScreenMessage() string {
	if r.chatCorrectionMode() {
		return "Cursor proxy rejected a tool call. " + nativeChatCorrectionMessage(r.correctionToolNames())
	}
	return "Cursor proxy rejected a built-in tool call; retry with Solomon native tools: searchTools (discover deferred SDK), orchestrate (run workspace scripts), searchSkill, loadSkill."
}

func ChatProxyToolCorrectionMessageForTest(blocked, names []string) string {
	catalog := map[string]struct{}{}
	for _, name := range names {
		catalog[name] = struct{}{}
	}
	return chatProxyToolCorrectionMessage(blocked, catalog)
}

func (r *Runtime) ProxyCorrectionScreenMessageForTest() string {
	return r.proxyCorrectionScreenMessage()
}
func (r *Runtime) StripCursorProxyInlineErrorsForTest(content string) (string, string) {
	return stripCursorProxyInlineErrorsWithCorrection(content, r.proxyToolCorrectionMessage)
}

func (r *Runtime) ToolInvocationCorrectionUserMsgForTest() string {
	return r.toolInvocationCorrectionUserMsg()
}
