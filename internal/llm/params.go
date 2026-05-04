package llm

import (
	"unicode/utf8"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func runeCount(s string) int64 {
	return int64(utf8.RuneCountInString(s))
}

func messageCharWeight(m chatstore.Message) int64 {
	n := runeCount(m.Content) + runeCount(m.ReasoningText)
	for _, tc := range m.ToolCalls {
		n += runeCount(tc.ID) + runeCount(tc.Name) + runeCount(tc.Arguments)
	}
	n += runeCount(m.ToolCallID)
	return n
}

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
	idx := lastUserMessageIndex(msgs)
	contextChars := runeCount(system)
	var userChars int64
	if idx < 0 {
		for _, m := range msgs {
			contextChars += messageCharWeight(m)
		}
		return apiPromptTokens, 0
	}
	userChars = messageCharWeight(msgs[idx])
	for i, m := range msgs {
		if i == idx {
			continue
		}
		contextChars += messageCharWeight(m)
	}
	totalChars := contextChars + userChars
	if totalChars <= 0 {
		return apiPromptTokens, 0
	}
	contextTok = apiPromptTokens * contextChars / totalChars
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

func ApplyMaxResponseTokens(cfg *config.Root, p *openai.ChatCompletionNewParams) {
	if cfg == nil || cfg.MaxResponseTokens < 1 {
		return
	}
	p.MaxCompletionTokens = param.NewOpt(int64(cfg.MaxResponseTokens))
}

func MessageParams(system string, msgs []chatstore.Message) []openai.ChatCompletionMessageParamUnion {
	out := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(system)}
	for _, m := range msgs {
		switch m.Role {
		case "assistant":
			if len(m.ToolCalls) > 0 {
				ap := openai.ChatCompletionAssistantMessageParam{}
				if m.Content != "" {
					ap.Content.OfString = param.NewOpt(m.Content)
				}
				for _, tc := range m.ToolCalls {
					ap.ToolCalls = append(ap.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   tc.ID,
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: tc.Arguments,
							},
						},
					})
				}
				out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &ap})
				continue
			}
			out = append(out, openai.AssistantMessage(m.Content))
		case "tool":
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
		case "user":
			out = append(out, openai.UserMessage(m.Content))
		default:
			out = append(out, openai.UserMessage(m.Role+": "+m.Content))
		}
	}
	return out
}

func ModelID(s string) shared.ChatModel {
	return shared.ChatModel(s)
}

func PopulateAssistantTurnUsage(dst *chatstore.Message, system string, msgsPrior []chatstore.Message, u UsageStats) {
	if dst == nil {
		return
	}
	_, usrTok, _ := UsagePromptParts(system, msgsPrior, u.PromptTokens, u.CachedPromptTokens)
	dst.UserPromptTokens = usrTok
	dst.ReasoningTokens = u.ReasoningTokens
	dst.ResponseTokens = u.ResponseTokens
	dst.TurnTotalTokens = u.TotalTokens
}
