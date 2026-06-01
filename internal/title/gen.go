package title

import (
	"context"
	"regexp"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

const maxCompletionTokens = 2048

var (
	titleFenceRe = regexp.MustCompile("(?s)```.*?```")
	titleTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func looksLikeToolMarkup(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "<tool") ||
		strings.Contains(l, "tool_calls") ||
		strings.Contains(l, "</tool>")
}

func cleanTitleText(s string) string {
	s = titleFenceRe.ReplaceAllString(s, " ")
	s = titleTagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func FromPrompt(ctx context.Context, backend llm.CompletionBackend, client openai.Client, cfg *config.Root, model string, userLine string) (string, error) {
	sys, err := prompt.RenderTitle(prompt.TitleData{Language: cfg.EffectiveResponseLanguage(), DisableThinking: true})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "title RenderTitle failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	user := "User message to name:\n" + userLine
	if backend != nil {
		t, err := backend.CompleteText(ctx, llm.SimpleCompletionRequest{
			Cfg:                   cfg,
			Model:                 model,
			System:                sys,
			User:                  user,
			ForceDisableReasoning: true,
		})
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "title completion failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return "", err
		}
		t = strings.TrimSpace(t)
		if t == "" || looksLikeToolMarkup(t) {
			return "", nil
		}
		t = cleanTitleText(t)
		if t == "" {
			return "", nil
		}
		return t, nil
	}
	p := openai.ChatCompletionNewParams{
		Model:               shared.ChatModel(model),
		Messages:            []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(sys), openai.UserMessage(user)},
		MaxCompletionTokens: param.NewOpt(int64(maxCompletionTokens)),
	}
	llm.ApplyReasoningDisable(&p)
	resp, err := client.Chat.Completions.New(ctx, p)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "title completions request failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	t := strings.TrimSpace(resp.Choices[0].Message.Content)
	if t == "" || looksLikeToolMarkup(t) {
		return "", nil
	}
	t = cleanTitleText(t)
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
	s = cleanTitleText(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	const maxRunes = 60
	if r := []rune(s); len(r) > maxRunes {
		s = string(r[:maxRunes])
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "untitled-chat"
	}
	return s
}
