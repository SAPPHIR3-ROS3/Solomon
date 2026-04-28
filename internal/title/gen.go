package title

import (
	"context"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
	"solomon/internal/config"
	"solomon/internal/llm"
	"solomon/internal/prompt"
)

func FromPrompt(ctx context.Context, client openai.Client, cfg *config.Root, model string, userLine string) (string, error) {
	sys, err := prompt.RenderTitle(prompt.TitleData{Language: cfg.EffectiveResponseLanguage()})
	if err != nil {
		return "", err
	}
	p := openai.ChatCompletionNewParams{
		Model:           shared.ChatModel(model),
		Messages:        []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(sys), openai.UserMessage("User message to name:\n" + userLine)},
		ReasoningEffort: shared.ReasoningEffort("none"),
	}
	llm.ApplyMaxResponseTokens(cfg, &p)
	resp, err := client.Chat.Completions.New(ctx, p)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	t := strings.TrimSpace(resp.Choices[0].Message.Content)
	if t == "" {
		return "", nil
	}
	return t, nil
}

func FallbackFromWords(userLine string) string {
	fields := strings.Fields(strings.TrimSpace(userLine))
	if len(fields) == 0 {
		return "untitled-chat"
	}
	n := 5
	if len(fields) < n {
		n = len(fields)
	}
	s := strings.Join(fields[:n], "-")
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, s)
}

func NormalizeSlug(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
