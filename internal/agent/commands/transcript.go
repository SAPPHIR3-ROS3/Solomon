package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
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
			writeTranscriptMessage(out, m, model)
			turnStart = i + 1
			continue
		}
		writeTranscriptMessage(out, m, model)
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

func writeTranscriptMessage(out io.Writer, m chatstore.Message, model string) {
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
		rtxt, cshow := chatstore.AssistantDisplayParts(m)
		rtxt = tooling.StripLegacyToolBlocks(rtxt)
		cshow = tooling.LegacyProseOutsideToolCalls(cshow)
		if rtxt != "" {
			fmt.Fprintf(out, "%s%s\n", prefix, termcolor.WrapThinking(rtxt))
		}
		if cshow != "" {
			fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapAssistant(model+":"), cshow)
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
			tooling.WriteToolDisplayLines(out, cpSeq, branch, tooling.FormatToolDisplayLines(tc.Name, json.RawMessage(tc.Arguments)))
		}
	case "tool":
		id := m.ToolCallID
		if id != "" {
			fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapThinking(fmt.Sprintf("[tool %s]", id)), truncateRunes(m.Content, 240))
		} else {
			fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapThinking("[tool]"), truncateRunes(m.Content, 240))
		}
	case "system":
		fmt.Fprint(out, prefix)
		termcolor.WriteSystem(out, m.Content)
	default:
	}
}
