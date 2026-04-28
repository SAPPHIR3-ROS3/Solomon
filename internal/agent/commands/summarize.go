package commands

import (
	"fmt"
	"io"
	"strings"
	"time"

	"solomon/internal/chatstore"
	"solomon/internal/llm"
	"solomon/internal/termcolor"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

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

func Summarize(d Deps) error {
	sess := d.Session()
	if sess == nil {
		return fmt.Errorf("no messages to summarize")
	}
	msgs := sess.Messages
	if len(msgs) == 0 {
		return fmt.Errorf("no messages to summarize")
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
		return err
	}
	if d.Cfg.UsageStatsEnabled() {
		fmt.Fprintln(d.Out, termcolor.UsageTokensLine(usage.PromptTokens, usage.ReasoningTokens, usage.ResponseTokens, usage.TotalTokens, usage.OutputTPS, usage.TTFTSecs, usage.PromptTPS))
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("empty summary from model")
	}
	var retainedBlock string
	if len(msgs) > 8 {
		retainedBlock = formatChatTranscript(msgs[len(msgs)-8:])
	} else {
		retainedBlock = formatChatTranscript(msgs)
	}
	body := compactSummaryBody(sep, summary, retainedBlock)
	sess.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
	sess.LastMessageAt = time.Now()
	if err := chatstore.WriteSession(d.ProjHex, sess); err != nil {
		return err
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	fmt.Fprintln(d.Out, body)
	fmt.Fprintln(d.Out, "Cronologia compattata: riassunto salvato; i messaggi precedenti sono stati sostituiti.")
	return nil
}
