package agentruntime

import (
	"encoding/json"
	"strings"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func (r *Runtime) cursorLegacyToolsActive() bool {
	return r != nil && r.Prov != nil && r.Prov.IsCursorAPI()
}

func (r *Runtime) legacyToolsEnabled() bool {
	if r.cursorLegacyToolsActive() {
		return true
	}
	return r != nil && r.Cfg != nil && r.Cfg.LegacyToolsEnabled()
}

func (r *Runtime) legacyToolsForced() bool {
	if r.cursorLegacyToolsActive() {
		return true
	}
	return r != nil && r.Cfg != nil && r.Cfg.LegacyToolsForceEnabled()
}

func (r *Runtime) allowedToolNames() (map[string]struct{}, error) {
	return r.allowedToolNamesForMode(r.Mode)
}

func (r *Runtime) allowedToolNamesForMode(mode string) (map[string]struct{}, error) {
	tools, err := agenttools.NativeToolParams(mode)
	if err != nil {
		return nil, err
	}
	if r != nil && r.MCP != nil && mode == r.Mode {
		tools = append(tools, r.MCP.OpenAITools()...)
	}
	names := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		if t.OfFunction == nil {
			continue
		}
		name := t.OfFunction.Function.Name
		if name == "" {
			continue
		}
		names[name] = struct{}{}
	}
	return names, nil
}

func (r *Runtime) ResolveTurnInvocations(turn llm.AssistantTurnResult, legacySW *tooling.LegacyStreamWriter) (invs []tooling.Invocation, toolIDs []string, rejectNative bool, malformed error) {
	if r.legacyToolsForced() {
		if len(turn.ToolCalls) > 0 {
			return nil, nil, true, nil
		}
		return r.legacyInvocationsFromTurn(turn, legacySW)
	}
	if len(turn.ToolCalls) > 0 {
		for _, tc := range turn.ToolCalls {
			invs = append(invs, tooling.Invocation{Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
			toolIDs = append(toolIDs, tc.ID)
		}
		return invs, toolIDs, false, nil
	}
	if r.legacyToolsEnabled() {
		return r.legacyInvocationsFromTurn(turn, legacySW)
	}
	return nil, nil, false, nil
}

func (r *Runtime) legacyInvocationsFromTurn(turn llm.AssistantTurnResult, legacySW *tooling.LegacyStreamWriter) (invs []tooling.Invocation, toolIDs []string, rejectNative bool, malformed error) {
	allowed, err := r.allowedToolNames()
	if err != nil {
		return nil, nil, false, err
	}
	if legacySW != nil && len(legacySW.Invocations()) > 0 {
		invs = legacySW.Invocations()
	} else if legacySW != nil && legacySW.HasOpenToolCalls() {
		return nil, nil, false, tooling.ErrMalformedLegacyTool
	} else {
		extracted, extractErr := tooling.ExtractToolInvocations(turn.Content)
		if extractErr != nil {
			return nil, nil, false, extractErr
		}
		invs = extracted
	}
	if err := tooling.ValidateInvocationNames(invs, allowed); err != nil {
		return nil, nil, false, err
	}
	for range invs {
		toolIDs = append(toolIDs, "")
	}
	return invs, toolIDs, false, nil
}

func (r *Runtime) stripCursorLegacyToolCallsFromSession() {
	if r == nil {
		return
	}
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		for i := range s.Messages {
			m := &s.Messages[i]
			if m.Role != "assistant" || len(m.ToolCalls) == 0 {
				continue
			}
			var kept []chatstore.ToolCall
			for _, tc := range m.ToolCalls {
				if strings.TrimSpace(tc.ID) != "" {
					kept = append(kept, tc)
				}
			}
			if len(kept) == 0 {
				m.ToolCalls = nil
			} else {
				m.ToolCalls = kept
			}
		}
	})
	_ = r.persistSession()
}

func (r *Runtime) syncLegacyToolCallsToLastAssistant(invs []tooling.Invocation) {
	if r == nil || len(invs) == 0 {
		return
	}
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil || len(s.Messages) == 0 {
			return
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role != "assistant" {
				continue
			}
			m := &s.Messages[i]
			if len(m.ToolCalls) > 0 {
				return
			}
			for _, inv := range invs {
				m.ToolCalls = append(m.ToolCalls, chatstore.ToolCall{
					Name:      inv.Name,
					Arguments: string(inv.Args),
				})
			}
			m.Content = tooling.LegacyProseOutsideToolCalls(m.Content)
			return
		}
	})
}
