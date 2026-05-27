package agentruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

const legacyToolJSONCorrectionUserMsg = "Your previous reply contained a malformed <tool_calls> block. Use exactly this shape with valid JSON in each <args> tag:\n<tool_calls>\n<tool name=\"TOOL_NAME\">\n<intent>brief purpose</intent>\n<args>{\"key\":\"value\"}</args>\n</tool>\n</tool_calls>\nSend a corrected block only, or continue without tools if you meant plain text."

func newLegacyStreamWriter(out io.Writer, enabled bool, allowed map[string]struct{}) (*tooling.LegacyStreamWriter, io.Writer) {
	if !enabled {
		return nil, out
	}
	lsw := tooling.NewLegacyStreamWriter(out, nil, allowed)
	return lsw, lsw
}

func (r *Runtime) stampAssistantToolCallCheckpoint(toolIdx, cpSeq int, branchKey string) {
	r.mutateSession(func(s *chatstore.Session) {
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role != "assistant" {
				continue
			}
			if toolIdx >= len(s.Messages[i].ToolCalls) {
				return
			}
			tc := &s.Messages[i].ToolCalls[toolIdx]
			tc.CheckpointSeq = cpSeq
			tc.CpSeqSet = true
			tc.CheckpointBranchKey = branchKey
			return
		}
	})
}

func (r *Runtime) printToolInvocation(toolIdx int, name string, rawArgs json.RawMessage) int {
	var cpSeq int
	var branchKey string
	r.mutateSession(func(s *chatstore.Session) {
		cpSeq = checkpoint.Bump(s)
		branchKey = s.CheckpointBranchSuffix
	})
	r.stampAssistantToolCallCheckpoint(toolIdx, cpSeq, branchKey)
	if intent := tooling.ExtractToolIntent(rawArgs); intent != "" {
		fmt.Fprintf(r.Out, "%s%s\n", checkpoint.FormatCheckpointPrefix(cpSeq, branchKey), termcolor.WrapThinking(intent))
	}
	tooling.WriteToolDisplayLines(r.Out, cpSeq, branchKey, formatToolDisplayLines(name, rawArgs))
	return cpSeq
}

const legacyNativeToolRejectedUserMsg = "Native API tool_calls are disabled because legacy tools force is ON. Do not use function calling. Emit tool invocations only inside a <tool_calls> XML block as described in the system prompt."

func (r *Runtime) handleRejectedNativeToolCall() error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, "Legacy tools force: native API tool_calls were ignored. Use <tool_calls> XML in assistant text.")
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: legacyNativeToolRejectedUserMsg}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func (r *Runtime) handleMalformedLegacyTool(err error) error {
	if !r.machineMode() {
		termcolor.WriteSystem(r.Out, legacyToolScreenMessage(err))
		fmt.Fprintln(r.Out)
		flushWriter(r.Out)
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq := checkpoint.Bump(s)
		um := chatstore.Message{Role: "user", Content: legacyToolJSONCorrectionUserMsg}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		s.LastMessageAt = time.Now()
	})
	return r.persistSession()
}

func legacyToolScreenMessage(err error) string {
	return tooling.UserFacingLegacyToolError(err)
}

func isMalformedLegacyToolErr(err error) bool {
	return errors.Is(err, tooling.ErrMalformedLegacyTool) || errors.Is(err, tooling.ErrUnknownLegacyTool)
}

func formatToolDisplayLines(name string, rawArgs json.RawMessage) []string {
	return tooling.FormatToolDisplayLines(name, rawArgs)
}

func formatToolPlainLines(name string, rawArgs json.RawMessage) []string {
	colored := tooling.FormatToolDisplayLines(name, rawArgs)
	out := make([]string, len(colored))
	for i, line := range colored {
		out[i] = stripANSI(line)
	}
	return out
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
