package llm

import (
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
	"solomon/internal/chatstore"
	"solomon/internal/config"
)

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
			out = append(out, openai.AssistantMessage(m.Content))
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
