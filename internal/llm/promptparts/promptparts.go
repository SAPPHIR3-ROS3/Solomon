package promptparts

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
)

func lastUserMessageIndex(msgs []chatstore.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return i
		}
	}
	return -1
}

func PromptDisplaySplit(system string, msgs []chatstore.Message, apiPromptTokens int64) (contextTok int64, lastUserTok int64) {
	if apiPromptTokens <= 0 {
		return 0, 0
	}
	model := tokcount.DefaultModel
	idx := lastUserMessageIndex(msgs)
	tpm, _ := tokcount.MessageOverhead(model)
	contextW := int64(tpm) + tokcount.TextTokens(system, model)
	userW := int64(0)
	lastAsst := apitype.LastAssistantIndex(msgs)
	if idx < 0 {
		for i, m := range msgs {
			contextW += chatstore.MessageWireWeight(m, i == lastAsst, model)
		}
		return apiPromptTokens, 0
	}
	userW = chatstore.MessageWireWeight(msgs[idx], false, model)
	for i, m := range msgs {
		if i == idx {
			continue
		}
		contextW += chatstore.MessageWireWeight(m, i == lastAsst, model)
	}
	totalW := contextW + userW
	if totalW <= 0 {
		return apiPromptTokens, 0
	}
	contextTok = apiPromptTokens * contextW / totalW
	lastUserTok = apiPromptTokens - contextTok
	return contextTok, lastUserTok
}

func UsagePromptParts(system string, msgs []chatstore.Message, promptTokens int64, cachedPromptTokens int64) (contextTok int64, lastUserTok int64, contextEstimated bool) {
	if promptTokens <= 0 {
		return 0, 0, false
	}
	if cachedPromptTokens > 0 {
		cached := cachedPromptTokens
		if cached > promptTokens {
			cached = promptTokens
		}
		return cached, promptTokens - cached, false
	}
	ctx, usr := PromptDisplaySplit(system, msgs, promptTokens)
	return ctx, usr, true
}
