package agentruntime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

// recordRejectedToolCalls closes native tool calls that could not be dispatched
// (for example, because their intent was missing). The assistant message is
// persisted before dispatch validation, so without this terminal result the
// chat API would expose the call as running forever and the next model request
// would contain an assistant tool call with no matching tool message.
func (r *Runtime) recordRejectedToolCalls(turn llm.AssistantTurnResult, turnIndex int, reason string) {
	if r == nil || r.Session == nil || len(turn.ToolCalls) == 0 {
		return
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "tool call rejected"
	}
	payload := rejectedToolResultJSON(reason)
	recorded := make([]llm.AssistantToolCall, 0, len(turn.ToolCalls))

	r.mutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		assistantIndex := -1
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role == "assistant" {
				assistantIndex = i
				break
			}
		}
		if assistantIndex < 0 {
			return
		}

		completed := make(map[string]struct{}, len(turn.ToolCalls))
		for _, message := range s.Messages[assistantIndex+1:] {
			if message.Role == "user" {
				break
			}
			if message.Role == "tool" && message.ToolCallID != "" {
				completed[message.ToolCallID] = struct{}{}
			}
		}

		for _, call := range turn.ToolCalls {
			if strings.TrimSpace(call.ID) == "" {
				continue
			}
			if _, ok := completed[call.ID]; ok {
				continue
			}
			seq := checkpoint.Bump(s)
			message := chatstore.Message{Role: "tool", ToolCallID: call.ID, Content: payload}
			checkpoint.StampMsg(&message, s, seq)
			s.Messages = append(s.Messages, message)
			s.LastMessageAt = time.Now().UTC()
			completed[call.ID] = struct{}{}
			recorded = append(recorded, call)
		}
	})

	if len(recorded) == 0 {
		return
	}
	r.noteCIToolResult(map[string]any{"error": reason})
	r.persistSessionOrLog("rejectedToolResult")
	if r.machineMode() {
		for _, call := range recorded {
			r.ciEmit(cievents.ToolResult(turnIndex, call.ID, call.Name, []byte(payload), reason))
		}
	}
}

func rejectedToolResultJSON(reason string) string {
	payload, err := json.Marshal(map[string]string{"error": reason})
	if err != nil {
		return `{"error":"tool call rejected"}`
	}
	return string(payload)
}

func appendNestedRejectedToolResults(messages *[]chatstore.Message, sequence *int, calls []llm.AssistantToolCall, reason string) {
	if messages == nil || sequence == nil || len(calls) == 0 {
		return
	}
	payload := rejectedToolResultJSON(reason)
	completed := make(map[string]struct{}, len(calls))
	for _, message := range *messages {
		if message.Role == "tool" && message.ToolCallID != "" {
			completed[message.ToolCallID] = struct{}{}
		}
	}
	for _, call := range calls {
		if strings.TrimSpace(call.ID) == "" {
			continue
		}
		if _, ok := completed[call.ID]; ok {
			continue
		}
		*sequence = *sequence + 1
		message := chatstore.Message{Role: "tool", ToolCallID: call.ID, Content: payload}
		stampNestedMessageCheckpoint(&message, *sequence)
		*messages = append(*messages, message)
		completed[call.ID] = struct{}{}
	}
}
