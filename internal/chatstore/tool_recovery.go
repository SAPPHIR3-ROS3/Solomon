package chatstore

import "encoding/json"

func ModelToolContent(content string) string {
	var payload map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &payload) != nil {
		return content
	}
	if _, ok := payload["sdk_tool_calls"]; !ok {
		return content
	}
	delete(payload, "sdk_tool_calls")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return content
	}
	return string(encoded)
}

// RepairInterruptedToolCalls closes calls whose results were lost before a new
// user turn. It never executes a tool or claims that its side effects failed.
func RepairInterruptedToolCalls(s *Session) int {
	if s == nil {
		return 0
	}
	var repaired []Message
	count := 0
	for i := 0; i < len(s.Messages); {
		message := s.Messages[i]
		repaired = append(repaired, message)
		i++
		if message.Role != "assistant" || len(message.ToolCalls) == 0 {
			continue
		}
		completed := map[string]bool{}
		for i < len(s.Messages) && s.Messages[i].Role == "tool" {
			completed[s.Messages[i].ToolCallID] = true
			repaired = append(repaired, s.Messages[i])
			i++
		}
		for _, call := range message.ToolCalls {
			if call.ID == "" || completed[call.ID] {
				continue
			}
			repaired = append(repaired, Message{Role: "tool", ToolCallID: call.ID, Content: `{"error":"Tool execution was interrupted before its result was saved. The outcome is unknown; check the current state before retrying any action."}`})
			completed[call.ID] = true
			count++
		}
	}
	if count > 0 {
		s.Messages = repaired
	}
	return count
}
