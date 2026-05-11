package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

const summarizeSystemPromptTemplate = `You summarize technical conversations concisely.
Preserve important facts: decisions, file paths, commands, errors, and open tasks.
Match the language of the transcript.
Output only the summary text, without preamble or meta-commentary.`

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
			rtxt, cshow := chatstore.AssistantDisplayParts(m)
			if rtxt != "" {
				fmt.Fprintf(&b, "Assistant (reasoning):\n%s\n\n", rtxt)
			}
			if cshow != "" {
				fmt.Fprintf(&b, "Assistant:\n%s\n\n", cshow)
			}
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

func summarizePromptFromTemplate(transcript string) (string, string) {
	user, err := prompt.RenderSummarize(prompt.SummarizeData{Transcript: transcript})
	if err != nil {
		return summarizeSystemPromptTemplate, transcript
	}
	return summarizeSystemPromptTemplate, user
}

func summarizeToolObjective(tc chatstore.ToolCall) string {
	name := strings.TrimSpace(tc.Name)
	if name == "" {
		name = "tool"
	}
	args := strings.TrimSpace(compactJSONArgs(tc.Arguments))
	if args == "" {
		return fmt.Sprintf("Use %s to advance the task.", name)
	}
	return fmt.Sprintf("Use %s to advance the task with args: %s", name, truncateRunes(args, 140))
}

func appendRetainedEntry(entries *[]chatstore.Message, role, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if role == "assistant" && len(*entries) > 0 && (*entries)[len(*entries)-1].Role == "assistant" {
		(*entries)[len(*entries)-1].Content = strings.TrimSpace((*entries)[len(*entries)-1].Content + "\n\n" + content)
		return
	}
	*entries = append(*entries, chatstore.Message{Role: role, Content: content})
}

func formatRetainedMessages(msgs []chatstore.Message) string {
	var entries []chatstore.Message
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if strings.HasPrefix(strings.TrimSpace(m.Content), "tool_result(") {
				continue
			}
			appendRetainedEntry(&entries, "user", m.Content)
		case "assistant":
			var parts []string
			rtxt, cshow := chatstore.AssistantDisplayParts(m)
			if rtxt != "" {
				parts = append(parts, "Reasoning:\n"+rtxt)
			}
			if cshow != "" {
				parts = append(parts, cshow)
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, "Tool objective: "+summarizeToolObjective(tc))
			}
			appendRetainedEntry(&entries, "assistant", strings.Join(parts, "\n\n"))
		case "tool":
			continue
		default:
			appendRetainedEntry(&entries, m.Role, m.Content)
		}
	}
	var b strings.Builder
	for _, m := range entries {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "User:\n%s\n\n", m.Content)
		case "assistant":
			fmt.Fprintf(&b, "Assistant:\n%s\n\n", m.Content)
		default:
			fmt.Fprintf(&b, "%s:\n%s\n\n", m.Role, m.Content)
		}
	}
	return b.String()
}

func compactSummaryBody(sep, summaryLLM, retainedBlock string) string {
	var b strings.Builder
	color := termcolor.WrapContext
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n%s\n\n", color(sep), color("[Conversation summary]"), color(sep), color(summaryLLM))
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n%s\n\n%s\n", color(sep), color("[Retained messages]"), color(sep), color(retainedBlock), color(sep))
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
	fmt.Fprintln(d.Out, "Summarizing…")
	transcript := formatChatTranscript(msgs)
	sys, userPrompt := summarizePromptFromTemplate(transcript)
	params := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(d.Model()),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(userPrompt),
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
		fmt.Fprintln(d.Out, termcolor.UsageTokensLine(ctxTok, usrTok, usage.ReasoningTokens, usage.ResponseTokens, usage.TotalTokens, usage.OutputTPS, usage.TTFTSecs, usage.PromptTPS, ctxEst, usage.TurnWallSecs))
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		logging.Log(logging.WARNING_LOG_LEVEL, "/summarize empty summary from model")
		return "", fmt.Errorf("empty summary from model")
	}
	var retainedBlock string
	if len(msgs) > 8 {
		retainedBlock = formatRetainedMessages(msgs[len(msgs)-8:])
	} else {
		retainedBlock = formatRetainedMessages(msgs)
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
	sess.MainOrphans = nil
	sess.CheckpointBranchSuffix = ""
	sess.ForkChildCount = nil
	sess.CheckpointLast = -1
	sess.CheckpointCP0 = true
	sess.LastCommitOID = ""
	sess.LastMessageAt = time.Now()
	chatstore.RepairSessionMalformedImages(sess)
	if err := chatstore.WriteSession(d.ProjHex, sess); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "/summarize persist compacted session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	fmt.Fprintln(d.Out, body)
	fmt.Fprintln(d.Out, "History compacted: summary saved; previous messages have been replaced.")
	return nil
}
