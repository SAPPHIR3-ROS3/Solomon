package checkpoint

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
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
