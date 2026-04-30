package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/llm"
	"solomon/internal/logging"
	"solomon/internal/termcolor"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

func Threshold(d Deps, parts []string) error {
	if len(parts) < 2 {
		fmt.Fprintf(d.Out, "compaction_threshold_tokens=%d (auto /summarize when prompt_tokens >= this after an assistant reply; usage must be reported by the API)\n", d.CompactionThresholdTokens())
		return nil
	}
	n, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return err
	}
	if n < config.MinCompactionThresholdTokens {
		return fmt.Errorf("threshold must be >= %d", config.MinCompactionThresholdTokens)
	}
	d.SetCompactionThresholdTokens(n)
	fmt.Fprintf(d.Out, "compaction_threshold_tokens=%d\n", n)
	return nil
}

func formatChatTranscript(msgs []chatstore.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "User:\n%s\n\n", m.Content)
		case "assistant":
			fmt.Fprintf(&b, "Assistant:\n%s\n\n", m.Content)
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					fmt.Fprintf(&b, "  [tool_call %s] %s(%s)\n", tc.ID, tc.Name, tc.Arguments)
				}
				fmt.Fprintf(&b, "\n")
			}
		case "tool":
			fmt.Fprintf(&b, "Tool[%s]:\n%s\n\n", m.ToolCallID, m.Content)
		default:
			fmt.Fprintf(&b, "%s:\n%s\n\n", m.Role, m.Content)
		}
	}
	return b.String()
}

func compactSummaryBody(sep, summaryLLM, retainedBlock string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n[Conversation summary]\n%s\n\n%s\n\n", sep, sep, summaryLLM)
	fmt.Fprintf(&b, "%s\n[Messaggi conservati]\n%s\n\n%s\n\n%s\n", sep, sep, retainedBlock, sep)
	return b.String()
}

func SummarizeBody(d Deps) (string, error) {
	sess := d.Session()
	if sess == nil {
		return "", fmt.Errorf("no messages to summarize")
	}
	msgs := sess.Messages
	if len(msgs) == 0 {
		return "", fmt.Errorf("no messages to summarize")
	}
	fmt.Fprintln(d.Out, "Riassunto in corso…")
	transcript := formatChatTranscript(msgs)
	sys := `You summarize technical conversations concisely. Preserve important facts: decisions, file paths, commands, errors, and open tasks. Match the language of the transcript. Output only the summary text, without preamble or meta-commentary.`
	params := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(d.Model()),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(transcript),
		},
		ReasoningEffort: d.Cfg.GlobalReasoningEffort(),
	}
	llm.ApplyMaxResponseTokens(d.Cfg, &params)
	const sep = "================================================================================"
	summary, usage, err := llm.StreamText(d.Ctx, d.Client, params, io.Discard, llm.StreamOpts{})
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "/summarize StreamText failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	if d.Cfg.UsageStatsEnabled() {
		ms := []chatstore.Message{{Role: "user", Content: transcript}}
		ctxTok, usrTok, ctxEst := llm.UsagePromptParts(sys, ms, usage.PromptTokens, usage.CachedPromptTokens)
		fmt.Fprintln(d.Out, termcolor.UsageTokensLine(ctxTok, usrTok, usage.ReasoningTokens, usage.ResponseTokens, usage.TotalTokens, usage.OutputTPS, usage.TTFTSecs, usage.PromptTPS, ctxEst))
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		logging.Log(logging.WARNING_LOG_LEVEL, "/summarize empty summary from model")
		return "", fmt.Errorf("empty summary from model")
	}
	var retainedBlock string
	if len(msgs) > 8 {
		retainedBlock = formatChatTranscript(msgs[len(msgs)-8:])
	} else {
		retainedBlock = formatChatTranscript(msgs)
	}
	return compactSummaryBody(sep, summary, retainedBlock), nil
}

func Summarize(d Deps) error {
	body, err := SummarizeBody(d)
	if err != nil {
		return err
	}
	sess := d.Session()
	sess.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
	sess.LastMessageAt = time.Now()
	if err := chatstore.WriteSession(d.ProjHex, sess); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "/summarize persist compacted session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	fmt.Fprintln(d.Out, body)
	fmt.Fprintln(d.Out, "Cronologia compattata: riassunto salvato; i messaggi precedenti sono stati sostituiti.")
	return nil
}
