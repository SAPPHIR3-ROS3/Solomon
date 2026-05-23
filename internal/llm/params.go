package llm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
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

func ApplyReasoningDisable(p *openai.ChatCompletionNewParams) {
	if p == nil {
		return
	}
	p.ReasoningEffort = shared.ReasoningEffort("none")
	applyReasoningDisableExtras(p)
}

func ApplyChatReasoning(cfg *config.Root, p *openai.ChatCompletionNewParams, forceDisable bool) {
	if p == nil {
		return
	}
	if forceDisable {
		ApplyReasoningDisable(p)
		return
	}
	if cfg == nil {
		return
	}
	if eff := cfg.GlobalReasoningEffort(); eff != "" {
		p.ReasoningEffort = eff
	}
	if cfg.ReasoningEffortIsNone() {
		applyReasoningDisableExtras(p)
	}
}

func applyReasoningDisableExtras(p *openai.ChatCompletionNewParams) {
	p.SetExtraFields(map[string]any{
		"enable_thinking": false,
		"chat_template_kwargs": map[string]any{
			"enable_thinking": false,
		},
	})
}

var imgPlaceholderRe = regexp.MustCompile(`\[img-(\d+)\]`)

// ImagePlaceholder returns the placeholder string for a given image sequence number.
func ImagePlaceholder(seq int) string {
	return fmt.Sprintf("[img-%d]", seq)
}

// MessageParams builds OpenAI API message params from chatstore messages.
// If imageFiles is non-nil and contains entries for [img-<n>] placeholders
// found in user messages, those placeholders are replaced with image content parts.
func MessageParams(system string, msgs []chatstore.Message, imageFiles map[int]string) []openai.ChatCompletionMessageParamUnion {
	msgs = MessagesForAPI(msgs)
	lastAsst := LastAssistantIndex(msgs)
	out := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(system)}
	for i, m := range msgs {
		switch m.Role {
		case "assistant":
			if len(m.ToolCalls) > 0 {
				ap := openai.ChatCompletionAssistantMessageParam{}
				if m.Content != "" {
					ap.Content.OfString = param.NewOpt(m.Content)
				}
				if i == lastAsst {
					if rt := strings.TrimSpace(m.ReasoningText); rt != "" {
						ap.SetExtraFields(map[string]any{"reasoning_content": rt})
					}
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
			if i == lastAsst {
				if rt := strings.TrimSpace(m.ReasoningText); rt != "" {
					ap := openai.ChatCompletionAssistantMessageParam{}
					ap.Content.OfString = param.NewOpt(m.Content)
					ap.SetExtraFields(map[string]any{"reasoning_content": rt})
					out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &ap})
					continue
				}
			}
			out = append(out, openai.AssistantMessage(m.Content))
		case "tool":
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
		case "user":
			content := m.Content
			if strings.TrimSpace(m.APIContent) != "" {
				content = m.APIContent
			}
			parts := BuildUserContentParts(content, imageFiles)
			if len(parts) == 0 {
				out = append(out, openai.UserMessage(""))
				continue
			}
			if len(parts) == 1 && parts[0].OfText != nil {
				out = append(out, openai.UserMessage(*parts[0].GetText()))
				continue
			}
			out = append(out, openai.UserMessage(parts))
		default:
			out = append(out, openai.UserMessage(m.Role+": "+m.Content))
		}
	}
	return out
}

func BuildUserContentParts(content string, imageFiles map[int]string) []openai.ChatCompletionContentPartUnionParam {
	content = chatstore.StripUnresolvedImgPlaceholders(content, imageFiles)
	if !imgPlaceholderRe.MatchString(content) {
		return []openai.ChatCompletionContentPartUnionParam{openai.TextContentPart(content)}
	}
	// Shortcut: content is just a single image tag with no surrounding text.
	trimmed := strings.TrimSpace(content)
	if m := imgPlaceholderRe.FindStringSubmatch(trimmed); m != nil && trimmed == m[0] {
		seq := atoi(m[1])
		if path, ok := imageFiles[seq]; ok {
			if part := imageContentPartFromFile(path); part != nil {
				return []openai.ChatCompletionContentPartUnionParam{*part}
			}
		}
	}
	var parts []openai.ChatCompletionContentPartUnionParam
	idx := 0
	for _, m := range imgPlaceholderRe.FindAllStringSubmatchIndex(content, -1) {
		if m[0] > idx {
			parts = append(parts, openai.TextContentPart(content[idx:m[0]]))
		}
		seq := atoi(content[m[2]:m[3]])
		idx = m[1]
		if path, ok := imageFiles[seq]; ok {
			if part := imageContentPartFromFile(path); part != nil {
				parts = append(parts, *part)
				continue
			}
		}
	}
	if idx < len(content) {
		parts = append(parts, openai.TextContentPart(content[idx:]))
	}
	return parts
}

var llmPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func imageMIMEForBinary(data []byte) (mime string, ok bool) {
	if len(data) < 3 {
		return "", false
	}
	switch {
	case len(data) >= len(llmPNG) && bytes.Equal(data[:len(llmPNG)], llmPNG):
		return "image/png", true
	case data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg", true
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif", true
	default:
		return "", false
	}
}

func imageContentPartFromFile(path string) *openai.ChatCompletionContentPartUnionParam {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	mime, ok := imageMIMEForBinary(data)
	if !ok {
		return nil
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	url := fmt.Sprintf("data:%s;base64,%s", mime, b64)
	return &openai.ChatCompletionContentPartUnionParam{
		OfImageURL: &openai.ChatCompletionContentPartImageParam{
			ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
				URL: url,
			},
		},
	}
}

// atoi is a simple ASCII-to-int for non-negative numbers in image tags.
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// JumpLeftOverImgTag treats an [img-<n>] tag as a single atomic unit: if the cursor
// lies anywhere inside the tag (start < pos <= end), it jumps to start (before the tag).
// Spaces around the tag are never considered part of the placeholder.
func JumpLeftOverImgTag(line []rune, pos int) int {
	if pos <= 0 {
		return -1
	}
	lineStr := string(line)
	for _, loc := range imgPlaceholderRe.FindAllStringSubmatchIndex(lineStr, -1) {
		start := utf8.RuneCountInString(lineStr[:loc[0]])
		end := utf8.RuneCountInString(lineStr[:loc[1]])
		if pos > start && pos <= end {
			return start
		}
	}
	return -1
}

// JumpRightOverImgTag treats an [img-<n>] tag as a single atomic unit: if the cursor
// lies anywhere inside the tag (start <= pos < end), it jumps to end (after the tag).
// Spaces around the tag are never considered part of the placeholder.
func JumpRightOverImgTag(line []rune, pos int) int {
	if pos >= len(line) {
		return -1
	}
	lineStr := string(line)
	for _, loc := range imgPlaceholderRe.FindAllStringSubmatchIndex(lineStr, -1) {
		start := utf8.RuneCountInString(lineStr[:loc[0]])
		end := utf8.RuneCountInString(lineStr[:loc[1]])
		if pos >= start && pos < end {
			return end
		}
	}
	return -1
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
