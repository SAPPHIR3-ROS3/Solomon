package checkpoint

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
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

// FullCheckpointID represents a parsed checkpoint identifier like "#006a".
type FullCheckpointID struct {
	Seq   int
	Suffix string // e.g. "a", "b", "" (main branch)
	Raw   string  // original user input for display
}

// ParseFullCheckpointID parses strings like "#006a", "006a", "#010", "010".
func ParseFullCheckpointID(raw string) (*FullCheckpointID, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty checkpoint ID")
	}
	raw = strings.TrimPrefix(raw, "#")
	// Split into numeric prefix and optional letter suffix.
	seqStr := raw
	suffix := ""
	for i := 0; i < len(seqStr); i++ {
		c := seqStr[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			seqStr = seqStr[:i]
			suffix = strings.ToLower(seqStr[i:])
			break
		}
	}
	n, err := strconv.Atoi(seqStr)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid checkpoint ID %q: bad sequence number", raw)
	}
	return &FullCheckpointID{Seq: n, Suffix: suffix, Raw: raw}, nil
}

// SplitAtFullID splits messages at the first message whose CheckpointSeq and
// CheckpointBranchKey match the given FullCheckpointID.  If suffix is empty it
// matches any branch key (backward-compatible with plain numeric IDs).
func SplitAtFullID(msgs []chatstore.Message, id *FullCheckpointID) (keep, drop []chatstore.Message, err error) {
	idx := -1
	for i, m := range msgs {
		if m.CheckpointSeq != id.Seq {
			continue
		}
		if id.Suffix == "" || m.CheckpointBranchKey == id.Suffix {
			idx = i
			break
		}
	}
	if idx < 0 {
		tag := FormatCheckpointTag(id.Seq, id.Suffix)
		return nil, nil, fmt.Errorf("checkpoint %s not found in transcript", tag)
	}
	return append([]chatstore.Message(nil), msgs[:idx+1]...), append([]chatstore.Message(nil), msgs[idx+1:]...), nil
}
