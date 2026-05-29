package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
)

func SlashGoto(d Deps, parts []string) error {
	if d.CheckpointGoto == nil {
		return fmt.Errorf("/goto unavailable")
	}
	if len(parts) < 2 {
		return fmt.Errorf("usage: /goto <checkpoint-id>")
	}
	raw := strings.TrimSpace(parts[1])
	id, err := checkpoint.ParseFullCheckpointID(raw)
	if err != nil {
		return fmt.Errorf("usage: /goto <checkpoint-id> (e.g. 5, #006a)")
	}
	return d.CheckpointGoto(id)
}

func SlashCheckpointAck(d Deps) {
	s := d.Session()
	if s == nil {
		PrintSystem(d.Out, "(no session)")
		return
	}
	if s.CheckpointLast >= 0 {
		PrintSystemf(d.Out, "checkpoint acknowledgment %s", checkpoint.FormatCheckpointTag(s.CheckpointLast, s.CheckpointBranchSuffix))
	} else {
		PrintSystem(d.Out, "checkpoint acknowledgment (none yet)")
	}
}
