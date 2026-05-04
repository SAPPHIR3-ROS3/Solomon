package checkpoint

import (
	"fmt"

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
