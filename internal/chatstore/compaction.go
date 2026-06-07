package chatstore

import "time"

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

func cloneMainOrphans(segs []MainOrphanSegment) []MainOrphanSegment {
	if len(segs) == 0 {
		return nil
	}
	out := make([]MainOrphanSegment, len(segs))
	for i, seg := range segs {
		out[i] = MainOrphanSegment{
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
		MainOrphans:            cloneMainOrphans(sess.MainOrphans),
		LastCommitOID:          sess.LastCommitOID,
	})
}

func ApplyCompaction(sess *Session, body string, at time.Time) {
	ArchiveUncompactedState(sess, at)
	sess.Messages = []Message{{Role: "assistant", Content: body}}
	sess.MainOrphans = nil
	sess.CheckpointBranchSuffix = ""
	sess.ForkChildCount = nil
	sess.CheckpointLast = -1
	sess.CheckpointCP0 = true
	sess.LastCommitOID = ""
	sess.LastMessageAt = at
	RepairSessionMalformedImages(sess)
}
