package llm

import (
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/openai/openai-go/v2"
)

func ApplyProviderTurnParams(proto Protocol, cfg *config.Root, openaiParams *openai.ChatCompletionNewParams, forceDisable bool) {
	if openaiParams == nil {
		return
	}
	switch proto {
	case ProtocolAnthropic:
		return
	default:
		ApplyChatReasoning(cfg, openaiParams, forceDisable)
		ApplyMaxResponseTokens(cfg, openaiParams)
	}
}

func ApplyProviderSimpleParams(proto Protocol, cfg *config.Root, openaiParams *openai.ChatCompletionNewParams, forceDisable bool) {
	if openaiParams == nil {
		return
	}
	switch proto {
	case ProtocolAnthropic:
		return
	default:
		if forceDisable {
			ApplyReasoningDisable(openaiParams)
		}
		ApplyMaxResponseTokens(cfg, openaiParams)
	}
}
