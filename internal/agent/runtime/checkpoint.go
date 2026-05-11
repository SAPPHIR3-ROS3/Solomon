package agentruntime

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

func (r *Runtime) ApplyGotoCheckpoint(id *checkpoint.FullCheckpointID) error {
	keep, drop, err := checkpoint.SplitAtFullID(r.Session.Messages, id)
	if err != nil {
		return err
	}
	dropCopy := append([]chatstore.Message(nil), drop...)
	r.Session.MainOrphans = append(r.Session.MainOrphans, chatstore.MainOrphanSegment{
		ForkAtInclusive: id.Seq,
		Messages:        dropCopy,
	})
	r.Session.Messages = keep
	r.Session.CheckpointBranchSuffix = checkpoint.NextForkSuffix(r.Session, id.Seq)
	chatstore.FinishSessionLoad(r.Session)
	cmds := r.slashDeps(context.Background())
	if err := commands.Clear(cmds); err != nil {
		return err
	}
	commands.WriteLabeledTranscript(r.Out, r.Session.Messages, r.Model)
	tag := checkpoint.FormatCheckpointTag(id.Seq, id.Suffix)
	fmt.Fprintf(r.Out, "goto %s: transcript truncated (%d messages moved to orphan main).\n", tag, len(dropCopy))
	return r.persistSession()
}
