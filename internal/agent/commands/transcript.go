package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

func WriteLabeledTranscript(out io.Writer, msgs []chatstore.Message, model string) {
	for _, m := range msgs {
		prefix := ""
		if chatstore.MessageCheckpointTagVisible(m) {
			prefix = checkpoint.FormatLinePrefix(m.CheckpointSeq, m.CheckpointBranchKey)
		}
		switch m.Role {
		case "user":
			if strings.HasPrefix(m.Content, "tool_result(") {
				continue
			}
			fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapUser("You:"), m.Content)
		case "assistant":
			rtxt, cshow := chatstore.AssistantDisplayParts(m)
			if rtxt != "" {
				fmt.Fprintf(out, "%s%s\n", prefix, termcolor.WrapThinking(rtxt))
			}
			if cshow != "" {
				fmt.Fprintf(out, "%s%s %s\n", prefix, termcolor.WrapAssistant(model+":"), cshow)
			}
			for _, tc := range m.ToolCalls {
				args := compactJSONArgs(tc.Arguments)
				fmt.Fprintf(out, "%s%s\n", prefix, termcolor.WrapTool(fmt.Sprintf("Tool: %s(%s)", tc.Name, args)))
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
}
