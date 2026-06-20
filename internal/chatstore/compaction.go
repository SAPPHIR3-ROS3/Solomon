package chatstore

import (
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
	prev := 0
	if sess != nil {
		prev = len(sess.Messages)
	}
	ArchiveUncompactedState(sess, at)
	sess.Messages = []Message{{Role: "assistant", Content: body}}
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
