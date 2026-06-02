package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
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
	ctxTok, usrTok, reasonTok, respTok, totalTok, outputTPS, ttftSecs, promptTPS, turnWallSecs, ctxEst, ok := chatstore.StoredUsageLineForTurnRange(msgs, start, end)
	if !ok {
		return
	}
	fmt.Fprintln(out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, outputTPS, ttftSecs, promptTPS, ctxEst, turnWallSecs))
}

func writeTranscriptMessage(out io.Writer, msgs []chatstore.Message, idx int, model string) {
	m := msgs[idx]
	prefix := ""
	if chatstore.MessageCheckpointTagVisible(m) {
		prefix = checkpoint.FormatLinePrefix(m.CheckpointSeq, m.CheckpointBranchKey)
	}
	switch m.Role {
	case "user":
		if strings.HasPrefix(m.Content, "tool_result(") {
			return
		}
		fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapUser("You:"), m.Content)
	case "assistant":
		if strings.Contains(m.Content, "[Conversation summary]") {
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
			if intent := tooling.ExtractToolIntent(json.RawMessage(tc.Arguments)); intent != "" {
				fmt.Fprintf(out, "%s%s\n", checkpoint.FormatCheckpointPrefix(cpSeq, branch), termcolor.WrapThinking(intent))
			}
			tooling.WriteToolDisplayLines(out, cpSeq, branch, tooling.FormatToolDisplayLines(tc.Name, json.RawMessage(tc.Arguments)))
		}
	case "tool":
		toolName := toolNameForResult(msgs, idx)
		lines := tooling.FormatToolResultDisplayLines(toolName, m.Content)
		if len(lines) == 0 {
			return
		}
		for _, line := range lines {
			fmt.Fprintf(out, "%s%s\n", prefix, line)
		}
	case "system":
		fmt.Fprint(out, prefix)
		termcolor.WriteSystem(out, m.Content)
	default:
	}
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
