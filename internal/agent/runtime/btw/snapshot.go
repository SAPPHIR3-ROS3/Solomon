package btw

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

func CompleteMessages(msgs []chatstore.Message) []chatstore.Message {
	out := append([]chatstore.Message(nil), msgs...)
	for len(out) > 0 {
		idx := len(out) - 1
		last := out[idx]
		if last.Role == "assistant" && len(last.ToolCalls) > 0 && !assistantToolsComplete(out, idx) {
			out = out[:idx]
			continue
		}
		break
	}
	return out
}

func assistantToolsComplete(msgs []chatstore.Message, astIdx int) bool {
	pending := map[string]struct{}{}
	for _, tc := range msgs[astIdx].ToolCalls {
		if id := tc.ID; id != "" {
			pending[id] = struct{}{}
		}
	}
	if len(pending) == 0 {
		return true
	}
	for i := astIdx + 1; i < len(msgs); i++ {
		if msgs[i].Role == "tool" {
			delete(pending, msgs[i].ToolCallID)
		}
	}
	return len(pending) == 0
}
