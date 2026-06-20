package checkpoint

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func SplitAtInclusiveDisplay(msgs []chatstore.Message, displayN int) (keep, drop []chatstore.Message, err error) {
	idx := -1
	for i, m := range msgs {
		if m.CheckpointSeq == displayN {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, nil, fmt.Errorf("checkpoint #%03d not found in transcript", displayN)
	}
	return append([]chatstore.Message(nil), msgs[:idx+1]...), append([]chatstore.Message(nil), msgs[idx+1:]...), nil
}

type FullCheckpointID struct {
	Seq    int
	Suffix string
	Raw    string
}

func ParseFullCheckpointID(raw string) (*FullCheckpointID, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty checkpoint ID")
	}
	hadHash := strings.HasPrefix(raw, "#")
	raw = strings.TrimPrefix(raw, "#")
	if len(raw) == 0 && hadHash {
		return nil, fmt.Errorf("empty checkpoint ID after #")
	}
	seqStr := raw
	suffix := ""
	for i := 0; i < len(seqStr); i++ {
		c := seqStr[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			suffix = strings.ToLower(seqStr[i:])
			seqStr = seqStr[:i]
			break
		}
	}
	for j := 0; j < len(suffix); j++ {
		c := suffix[j]
		if !(c >= 'a' && c <= 'z') {
			return nil, fmt.Errorf("invalid checkpoint ID %q: invalid suffix (only letters allowed)", raw)
		}
	}
	n, err := strconv.Atoi(seqStr)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid checkpoint ID %q: bad sequence number", raw)
	}
	return &FullCheckpointID{Seq: n, Suffix: suffix, Raw: raw}, nil
}

func messageMatchesCheckpointID(m chatstore.Message, id *FullCheckpointID) bool {
	if m.CheckpointSeq != id.Seq {
		return false
	}
	return m.CheckpointBranchKey == id.Suffix
}

func messageContainsToolCallCheckpoint(m chatstore.Message, id *FullCheckpointID) bool {
	for _, tc := range m.ToolCalls {
		if !tc.CpSeqSet {
			continue
		}
		if tc.CheckpointSeq == id.Seq && tc.CheckpointBranchKey == id.Suffix {
			return true
		}
	}
	return false
}

func findCheckpointSplitIndex(msgs []chatstore.Message, id *FullCheckpointID) int {
	for i, m := range msgs {
		if messageMatchesCheckpointID(m, id) || messageContainsToolCallCheckpoint(m, id) {
			return i
		}
	}
	return -1
}

// SplitAtFullID splits messages at the first message whose CheckpointSeq and
// CheckpointBranchKey match the given FullCheckpointID exactly.  An empty
// suffix matches only messages on the main branch (CheckpointBranchKey == "").
// Tags printed on tool invocations (separate Bump per tool) are stored on
// ToolCalls; those match here via the parent assistant message index.
func SplitAtFullID(msgs []chatstore.Message, id *FullCheckpointID) (keep, drop []chatstore.Message, err error) {
	idx := findCheckpointSplitIndex(msgs, id)
	if idx < 0 {
		tag := FormatCheckpointTag(id.Seq, id.Suffix)
		return nil, nil, fmt.Errorf("checkpoint %s not found in transcript", tag)
	}
	return append([]chatstore.Message(nil), msgs[:idx+1]...), append([]chatstore.Message(nil), msgs[idx+1:]...), nil
}

func findCheckpointInBranches(branches []chatstore.BranchSegment, id *FullCheckpointID) int {
	for i, seg := range branches {
		if findCheckpointSplitIndex(seg.Messages, id) >= 0 {
			return i
		}
	}
	return -1
}

func findCheckpointSeqIndex(msgs []chatstore.Message, seq int) int {
	for i, m := range msgs {
		if m.CheckpointSeq == seq && chatstore.MessageCheckpointTagVisible(m) {
			return i
		}
	}
	return -1
}

func lastVisibleCheckpoint(msgs []chatstore.Message) int {
	max := -1
	for _, m := range msgs {
		if !chatstore.MessageCheckpointTagVisible(m) {
			continue
		}
		if m.CheckpointSeq > max {
			max = m.CheckpointSeq
		}
	}
	return max
}

func removeBranchAt(branches []chatstore.BranchSegment, idx int) []chatstore.BranchSegment {
	if idx < 0 || idx >= len(branches) {
		return branches
	}
	out := make([]chatstore.BranchSegment, 0, len(branches)-1)
	out = append(out, branches[:idx]...)
	out = append(out, branches[idx+1:]...)
	return out
}

func prefixThroughCheckpoint(active []chatstore.Message, branches []chatstore.BranchSegment, forkAt int) ([]chatstore.Message, []chatstore.BranchSegment, error) {
	prefix := append([]chatstore.Message(nil), active...)
	remaining := append([]chatstore.BranchSegment(nil), branches...)
	for {
		if idx := findCheckpointSeqIndex(prefix, forkAt); idx >= 0 {
			return prefix[:idx+1], remaining, nil
		}
		if len(prefix) == 0 {
			return nil, remaining, fmt.Errorf("checkpoint #%03d not found in transcript", forkAt)
		}
		lastCp := lastVisibleCheckpoint(prefix)
		bridged := false
		for i, seg := range remaining {
			if seg.ForkAtInclusive != lastCp {
				continue
			}
			prefix = append(prefix, seg.Messages...)
			remaining = append(remaining[:i], remaining[i+1:]...)
			bridged = true
			break
		}
		if !bridged {
			return nil, remaining, fmt.Errorf("checkpoint #%03d not found in transcript", forkAt)
		}
	}
}

// ResolveSessionGoto computes the active transcript and stored branch segments
// after jumping to id. It searches both the live branch and Branches so earlier
// truncated checkpoints remain reachable.
func ResolveSessionGoto(s *chatstore.Session, id *FullCheckpointID) (messages []chatstore.Message, branches []chatstore.BranchSegment, err error) {
	if s == nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "checkpoint goto nil session")
		return nil, nil, fmt.Errorf("nil session")
	}
	tag := FormatCheckpointTag(id.Seq, id.Suffix)

	if findCheckpointSplitIndex(s.Messages, id) >= 0 {
		keep, drop, splitErr := SplitAtFullID(s.Messages, id)
		if splitErr != nil {
			return nil, nil, splitErr
		}
		branches = append([]chatstore.BranchSegment(nil), s.Branches...)
		if len(drop) > 0 {
			branches = append(branches, chatstore.BranchSegment{
				ForkAtInclusive: id.Seq,
				Messages:        drop,
			})
		}
		return keep, branches, nil
	}

	segIdx := findCheckpointInBranches(s.Branches, id)
	if segIdx < 0 {
		logging.Log(logging.WARNING_LOG_LEVEL, "checkpoint goto not found", logging.LogOptions{Params: map[string]any{"checkpoint": tag}})
		return nil, nil, fmt.Errorf("checkpoint %s not found in transcript", tag)
	}
	targetSeg := s.Branches[segIdx]
	forkAt := targetSeg.ForkAtInclusive

	var activeBase []chatstore.Message
	var activeTail []chatstore.Message
	if idx := findCheckpointSeqIndex(s.Messages, forkAt); idx >= 0 {
		activeBase = append([]chatstore.Message(nil), s.Messages[:idx+1]...)
		activeTail = append([]chatstore.Message(nil), s.Messages[idx+1:]...)
	} else {
		activeBase = append([]chatstore.Message(nil), s.Messages...)
	}

	bridgeBranches := removeBranchAt(s.Branches, segIdx)
	prefix, remainingBranches, err := prefixThroughCheckpoint(activeBase, bridgeBranches, forkAt)
	if err != nil {
		return nil, nil, err
	}

	branchKeep, branchDrop, err := SplitAtFullID(targetSeg.Messages, id)
	if err != nil {
		return nil, nil, err
	}
	messages = append(prefix, branchKeep...)

	branches = remainingBranches
	if len(activeTail) > 0 {
		branches = append(branches, chatstore.BranchSegment{
			ForkAtInclusive: forkAt,
			Messages:        activeTail,
		})
	}
	if len(branchDrop) > 0 {
		branches = append(branches, chatstore.BranchSegment{
			ForkAtInclusive: id.Seq,
			Messages:        branchDrop,
		})
	}
	return messages, branches, nil
}

type RewindPlan struct {
	Target            *FullCheckpointID
	Messages          []chatstore.Message
	Branches          []chatstore.BranchSegment
	DroppedMsgs       int
	RemovedBranches   int
	RemovedBranchMsgs int
}

func countBranchMessages(segs []chatstore.BranchSegment) int {
	n := 0
	for _, seg := range segs {
		n += len(seg.Messages)
	}
	return n
}

func partitionBranches(branches []chatstore.BranchSegment, forkAt int) (kept, removed []chatstore.BranchSegment) {
	for _, seg := range branches {
		if seg.ForkAtInclusive >= forkAt {
			removed = append(removed, seg)
		} else {
			kept = append(kept, seg)
		}
	}
	return kept, removed
}

func PruneForkChildCount(m map[int]int, from int) map[int]int {
	if len(m) == 0 {
		return nil
	}
	out := make(map[int]int)
	for k, v := range m {
		if k < from {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PlanSessionRewind prepares a destructive rewind to id on the active branch only.
// Messages after id and every stored branch forked at id or later are dropped.
func PlanSessionRewind(s *chatstore.Session, id *FullCheckpointID) (*RewindPlan, error) {
	if s == nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "checkpoint rewind nil session")
		return nil, fmt.Errorf("nil session")
	}
	tag := FormatCheckpointTag(id.Seq, id.Suffix)
	if findCheckpointSplitIndex(s.Messages, id) < 0 {
		logging.Log(logging.WARNING_LOG_LEVEL, "checkpoint rewind not on current branch", logging.LogOptions{Params: map[string]any{"checkpoint": tag}})
		return nil, fmt.Errorf("checkpoint %s not found on current branch", tag)
	}
	curTag := FormatCheckpointTag(s.CheckpointLast, s.CheckpointBranchSuffix)
	if s.CheckpointLast >= 0 && id.Seq > s.CheckpointLast {
		return nil, fmt.Errorf("rewind only moves backward (current %s)", curTag)
	}
	keep, drop, err := SplitAtFullID(s.Messages, id)
	if err != nil {
		return nil, err
	}
	keptBranches, removedBranches := partitionBranches(s.Branches, id.Seq)
	removedBranchMsgs := countBranchMessages(removedBranches)
	if len(drop) == 0 && len(removedBranches) == 0 {
		return nil, fmt.Errorf("already at checkpoint %s", tag)
	}
	return &RewindPlan{
		Target:            id,
		Messages:          keep,
		Branches:          keptBranches,
		DroppedMsgs:       len(drop),
		RemovedBranches:   len(removedBranches),
		RemovedBranchMsgs: removedBranchMsgs,
	}, nil
}
