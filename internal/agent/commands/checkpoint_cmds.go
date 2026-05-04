package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
)

func SlashGoto(d Deps, parts []string) error {
	if d.CheckpointGoto == nil {
		return fmt.Errorf("/goto unavailable")
	}
	if len(parts) < 2 {
		return fmt.Errorf("usage: /goto <n>")
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || n < 0 {
		return fmt.Errorf("usage: /goto <n>")
	}
	return d.CheckpointGoto(n)
}

func SlashCheckpointAck(d Deps) {
	s := d.Session()
	if s == nil {
		fmt.Fprintln(d.Out, "(no session)")
		return
	}
	if s.CheckpointLast >= 0 {
		fmt.Fprintf(d.Out, "checkpoint acknowledgment %s\n", checkpoint.FormatCheckpointTag(s.CheckpointLast, s.CheckpointBranchSuffix))
	} else {
		fmt.Fprintln(d.Out, "checkpoint acknowledgment (none yet)")
	}
}
