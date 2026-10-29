package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/terminalmedia"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func WriteLabeledTranscript(out io.Writer, msgs []chatstore.Message, model string, showUsage bool) {
	if showUsage {
		chatstore.BackfillSessionAssistantUsage(msgs)
	}
	turnStart := 0
	for i := range msgs {
		m := msgs[i]
		if m.Role == "user" && !strings.HasPrefix(strings.TrimSpace(m.Content), "tool_result(") {
			if showUsage && i > turnStart {
				writeStoredUsageLine(out, msgs, turnStart, i)
			}
			writeTranscriptMessage(out, msgs, i, model)
			turnStart = i + 1
			continue
		}
		writeTranscriptMessage(out, msgs, i, model)
	}
	if showUsage && turnStart < len(msgs) {
		writeStoredUsageLine(out, msgs, turnStart, len(msgs))
	}
}

func writeStoredUsageLine(out io.Writer, msgs []chatstore.Message, start, end int) {
	ctxTok, usrTok, reasonTok, respTok, totalTok, cachedPromptTok, cacheCreationPromptTok, outputTPS, ttftSecs, promptTPS, turnWallSecs, ctxEst, ok := chatstore.StoredUsageLineForTurnRange(msgs, start, end)
	if !ok {
		return
	}
	fmt.Fprintln(out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, cachedPromptTok, cacheCreationPromptTok, outputTPS, ttftSecs, promptTPS, ctxEst, turnWallSecs))
}

func writeTranscriptMessage(out io.Writer, msgs []chatstore.Message, idx int, model string) {
	m := msgs[idx]
	model = transcriptAssistantLabel(m, model)
	prefix := ""
	if chatstore.MessageCheckpointTagVisible(m) {
		prefix = checkpoint.FormatLinePrefix(m.CheckpointSeq, m.CheckpointBranchKey)
	}
	switch m.Role {
	case "user":
		if strings.HasPrefix(m.Content, "tool_result(") {
			writeTranscriptMedia(out, m)
			return
		}
		fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapUser("You:"), m.Content)
	case "assistant":
		if messagePrecededByUser(msgs, idx) {
			termcolor.PrintChatSeparator(out)
		}
		if chatstore.IsCompactionSummary(m) {
			fmt.Fprintf(out, "%s%s\n", prefix, RenderCompactSummaryBody(m.Content))
			break
		}
		rtxt, cshow := chatstore.AssistantDisplayParts(m)
		rtxt = chatstore.StripAllImgPlaceholderLiterals(tooling.StripLegacyToolBlocks(rtxt))
		cshow = chatstore.StripAllImgPlaceholderLiterals(tooling.LegacyProseOutsideToolCalls(cshow))
		if rtxt != "" {
			fmt.Fprintf(out, "%s%s\n", prefix, termcolor.WrapThinking(rtxt))
		}
		if cshow != "" {
			fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapAssistant(model+":"), termcolor.ColorizeErrorLines(cshow))
			seen := terminalmedia.WriteAttachments(context.Background(), out, m.MediaPreviews)
			terminalmedia.WriteReferences(context.Background(), out, cshow, m.MediaRoot, seen)
		}
		toolCalls := m.ToolCalls
		if len(toolCalls) == 0 {
			if invs, err := tooling.ExtractToolInvocations(m.Content); err == nil && len(invs) > 0 {
				for _, inv := range invs {
					toolCalls = append(toolCalls, chatstore.ToolCall{Name: inv.Name, Arguments: string(inv.Args)})
				}
			}
		}
		for _, tc := range toolCalls {
			cpSeq, branch := m.CheckpointSeq, m.CheckpointBranchKey
			if tc.CpSeqSet {
				cpSeq, branch = tc.CheckpointSeq, tc.CheckpointBranchKey
			}
			tooling.WriteToolDisplayLines(out, cpSeq, branch, []string{termcolor.WrapThinking(tooling.ToolIntentDisplay(json.RawMessage(tc.Arguments)))})
			tooling.WriteToolDisplayLines(out, cpSeq, branch, tooling.FormatToolDisplayLines(tc.Name, json.RawMessage(tc.Arguments)))
		}
	case "tool":
		toolName := toolNameForResult(msgs, idx)
		lines := tooling.FormatToolResultDisplayLines(toolName, m.Content)
		if len(lines) == 0 {
			writeTranscriptMedia(out, m)
			return
		}
		continuation := ""
		if prefix != "" {
			continuation = checkpoint.FormatCheckpointContinuationPlain(m.CheckpointSeq, m.CheckpointBranchKey)
		}
		tooling.WriteToolDisplayLinesWithPrefixes(out, prefix, continuation, lines)
		writeTranscriptMedia(out, m)
	case "system":
		fmt.Fprint(out, prefix)
		termcolor.WriteSystem(out, m.Content)
	default:
	}
}

func messagePrecededByUser(msgs []chatstore.Message, idx int) bool {
	if idx <= 0 {
		return false
	}
	prev := msgs[idx-1]
	if prev.Role != "user" {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(prev.Content), "tool_result(")
}

func toolNameForResult(msgs []chatstore.Message, toolIdx int) string {
	id := strings.TrimSpace(msgs[toolIdx].ToolCallID)
	if id == "" {
		return ""
	}
	for j := toolIdx - 1; j >= 0; j-- {
		switch msgs[j].Role {
		case "tool":
			continue
		case "assistant":
			for _, tc := range msgs[j].ToolCalls {
				if tc.ID == id {
					return tc.Name
				}
			}
			return ""
		default:
			return ""
		}
	}
	return ""
}

// Prefer the recorded target when replaying a conversation after model changes.
func transcriptAssistantLabel(m chatstore.Message, fallback string) string {
	model := strings.TrimSpace(m.Model)
	if model == "" {
		model = strings.TrimSpace(fallback)
	}
	if model == "" {
		model = "Assistant"
	}
	if provider := strings.TrimSpace(m.Provider); provider != "" {
		model += " (" + provider + ")"
	}
	return model
}

func writeTranscriptMedia(out io.Writer, m chatstore.Message) {
	ctx := context.Background()
	seen := terminalmedia.WriteAttachments(ctx, out, m.ToolImages)
	raw := m.Content
	if strings.HasPrefix(raw, "tool_result(") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "tool_result("), ")")
	}
	var result any
	if json.Unmarshal([]byte(raw), &result) == nil {
		terminalmedia.WriteResult(ctx, out, result, m.MediaRoot, seen)
	}
}
