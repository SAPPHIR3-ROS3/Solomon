package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
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

func SlashRewind(d Deps, parts []string) error {
	if d.CheckpointRewind == nil {
		return fmt.Errorf("/rewind unavailable")
	}
	if len(parts) < 2 {
		return fmt.Errorf("usage: /rewind <checkpoint-id>")
	}
	raw := strings.TrimSpace(parts[1])
	id, err := checkpoint.ParseFullCheckpointID(raw)
	if err != nil {
		return fmt.Errorf("usage: /rewind <checkpoint-id> (e.g. 5, #006a)")
	}
	s := d.Session()
	if s == nil {
		return fmt.Errorf("no active session")
	}
	plan, err := checkpoint.PlanSessionRewind(s, id)
	if err != nil {
		return err
	}
	tag := checkpoint.FormatCheckpointTag(plan.Target.Seq, plan.Target.Suffix)
	warn := fmt.Sprintf("rewind to %s: will delete %d message(s) and %d alternate branch(es) (%d message(s)).", tag, plan.DroppedMsgs, plan.RemovedBranches, plan.RemovedBranchMsgs)
	termcolor.WriteRedBlock(d.Out, warn)
	ok, err := confirmYesNo(d, "Confirm rewind? [y/N]: ")
	if err != nil {
		return err
	}
	if !ok {
		PrintSystem(d.Out, "rewind cancelled.")
		return nil
	}
	return d.CheckpointRewind(plan)
}

func confirmYesNo(d Deps, prompt string) (bool, error) {
	line, err := config.ReadPromptLine(PromptIO(d), prompt)
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
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
