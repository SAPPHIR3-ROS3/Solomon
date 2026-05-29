package llm

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

func LastAssistantIndex(msgs []chatstore.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return i
		}
	}
	return -1
}

func MessageForAPI(m chatstore.Message, includeReasoning bool) chatstore.Message {
	if includeReasoning {
		return m
	}
	cp := m
	cp.ReasoningText = ""
	return cp
}

func MessagesForAPI(msgs []chatstore.Message) []chatstore.Message {
	last := LastAssistantIndex(msgs)
	out := make([]chatstore.Message, len(msgs))
	for i, m := range msgs {
		out[i] = MessageForAPI(m, i == last)
	}
	return out
}
