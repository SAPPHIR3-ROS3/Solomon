package agent

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
)

func (r *Runtime) checkpointStageProjAbs(absResolved string) {
	if checkpoint.SkipStagingIfRunningExecutable(absResolved) {
		return
	}
}

func (r *Runtime) ApplyGotoCheckpoint(targetDisplay int) error {
	keep, drop, err := checkpoint.SplitAtInclusiveDisplay(r.Session.Messages, targetDisplay)
	if err != nil {
		return err
	}
	dropCopy := append([]chatstore.Message(nil), drop...)
	r.Session.MainOrphans = append(r.Session.MainOrphans, chatstore.MainOrphanSegment{
		ForkAtInclusive: targetDisplay,
		Messages:        dropCopy,
	})
	r.Session.Messages = keep
	r.Session.CheckpointBranchSuffix = checkpoint.NextForkSuffix(r.Session, targetDisplay)
	chatstore.FinishSessionLoad(r.Session)
	cmds := r.slashDeps(context.Background())
	if err := commands.Clear(cmds); err != nil {
		return err
	}
	commands.WriteLabeledTranscript(r.Out, r.Session.Messages, r.Model)
	fmt.Fprintf(r.Out, "goto #%03d: transcript truncated (%d messages moved to orphan main).\n", targetDisplay, len(dropCopy))
	return r.persistSession()
}
