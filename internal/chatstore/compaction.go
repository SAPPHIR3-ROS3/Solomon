package chatstore

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func cloneMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if len(out[i].AnthropicThinking) > 0 {
			blocks := make([]json.RawMessage, len(out[i].AnthropicThinking))
			for j, block := range out[i].AnthropicThinking {
				blocks[j] = append(json.RawMessage(nil), block...)
			}
			out[i].AnthropicThinking = blocks
		}
		if len(out[i].ToolCalls) > 0 {
			out[i].ToolCalls = append([]ToolCall(nil), out[i].ToolCalls...)
		}
	}
	return out
}

func cloneBranches(segs []BranchSegment) []BranchSegment {
	if len(segs) == 0 {
		return nil
	}
	out := make([]BranchSegment, len(segs))
	for i, seg := range segs {
		out[i] = BranchSegment{
			ForkAtInclusive: seg.ForkAtInclusive,
			Messages:        cloneMessages(seg.Messages),
		}
	}
	return out
}

func cloneForkChildCount(m map[int]int) map[int]int {
	if len(m) == 0 {
		return nil
	}
	out := make(map[int]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func ArchiveUncompactedState(sess *Session, at time.Time) {
	if sess == nil || len(sess.Messages) == 0 {
		return
	}
	sess.UncompactedRaw = append(sess.UncompactedRaw, UncompactedDump{
		CompactAt:              at,
		Messages:               cloneMessages(sess.Messages),
		CheckpointLast:         sess.CheckpointLast,
		CheckpointCP0:          sess.CheckpointCP0,
		CheckpointBranchSuffix: sess.CheckpointBranchSuffix,
		ForkChildCount:         cloneForkChildCount(sess.ForkChildCount),
		Branches:               cloneBranches(sess.Branches),
		LastCommitOID:          sess.LastCommitOID,
	})
}

func ApplyCompaction(sess *Session, body string, at time.Time) {
	ApplyCompactionWithPendingMessages(sess, body, at, nil)
}

func ApplyCompactionWithPendingMessages(sess *Session, body string, at time.Time, pending []Message) {
	prev := 0
	if sess != nil {
		prev = len(sess.Messages)
	}
	if sess == nil {
		return
	}
	sess.Capabilities.Backfill(sess.Messages)
	ArchiveUncompactedState(sess, at)
	sess.Messages = []Message{{Role: "assistant", Content: body}}
	if restored, ok := sess.Capabilities.RestoreMessage(); ok {
		sess.Messages = append(sess.Messages, restored)
	}
	sess.Messages = append(sess.Messages, pending...)
	sess.Branches = nil
	sess.CheckpointBranchSuffix = ""
	sess.ForkChildCount = nil
	sess.CheckpointLast = -1
	sess.CheckpointCP0 = true
	sess.LastCommitOID = ""
	sess.LastMessageAt = at
	RepairSessionMalformedImages(sess)
	logging.Log(logging.INFO_LOG_LEVEL, "session compacted", logging.LogOptions{Params: map[string]any{"archived_messages": prev}})
}

// IsCompactionSummary recognizes the stored summary envelope, rather than
// mentions of its marker in ordinary assistant explanations or tool output.
func IsCompactionSummary(message Message) bool {
	if message.Role != "assistant" {
		return false
	}
	lines := strings.SplitN(strings.TrimSpace(message.Content), "\n", 4)
	if len(lines) < 4 || strings.TrimSpace(lines[1]) != "[Conversation summary]" {
		return false
	}
	separator := strings.TrimSpace(lines[0])
	if len(separator) < 3 || strings.TrimSpace(lines[2]) != separator {
		return false
	}
	first := []rune(separator)[0]
	for _, char := range separator {
		if char != first {
			return false
		}
	}
	return strings.ContainsRune("-=─━", first)
}

func ConversationSummaryText(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	var sections []string
	start := -1
	flush := func(end int) {
		if start < 0 || start >= end {
			return
		}
		from, to := start, end
		for from < to && summarySeparatorLine(lines[from]) {
			from++
		}
		for to > from && summarySeparatorLine(lines[to-1]) {
			to--
		}
		if text := strings.TrimSpace(strings.Join(lines[from:to], "\n")); text != "" {
			sections = append(sections, text)
		}
		start = -1
	}
	for index, line := range lines {
		switch strings.TrimSpace(line) {
		case "[Conversation summary]":
			flush(index)
			start = index + 1
		case "[Retained messages]":
			flush(index)
		}
	}
	flush(len(lines))
	if len(sections) == 0 {
		return strings.TrimSpace(body)
	}
	return sections[len(sections)-1]
}

func summarySeparatorLine(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	first := []rune(line)[0]
	if !strings.ContainsRune("-=─━", first) {
		return false
	}
	for _, char := range line {
		if char != first {
			return false
		}
	}
	return true
}
