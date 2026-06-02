package llm

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
)

func LastAssistantIndex(msgs []chatstore.Message) int {
	return apitype.LastAssistantIndex(msgs)
}

func MessageForAPI(m chatstore.Message, includeReasoning bool) chatstore.Message {
	return apitype.MessageForAPI(m, includeReasoning)
}

func MessagesForAPI(msgs []chatstore.Message) []chatstore.Message {
	return apitype.MessagesForAPI(msgs)
}
