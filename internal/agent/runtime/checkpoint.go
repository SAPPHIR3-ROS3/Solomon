package agentruntime

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
)

func (r *Runtime) checkpointStageProjAbs(absResolved string) {
	if checkpoint.SkipStagingIfRunningExecutable(absResolved) {
		return
	}
}

func (r *Runtime) ApplyGotoCheckpoint(id *checkpoint.FullCheckpointID) error {
	var splitErr error
	var dropCopy []chatstore.Message
	var truncLen int
	r.mutateSession(func(s *chatstore.Session) {
		keep, drop, err := checkpoint.SplitAtFullID(s.Messages, id)
		if err != nil {
			splitErr = err
			return
		}
		dropCopy = append([]chatstore.Message(nil), drop...)
		s.MainOrphans = append(s.MainOrphans, chatstore.MainOrphanSegment{
			ForkAtInclusive: id.Seq,
			Messages:        dropCopy,
		})
		s.Messages = keep
		s.CheckpointBranchSuffix = checkpoint.NextForkSuffix(s, id.Seq)
		chatstore.FinishSessionLoad(s)
		truncLen = len(dropCopy)
	})
	if splitErr != nil {
		return splitErr
	}
	cmds := r.slashDeps(context.Background())
	if err := commands.Clear(cmds); err != nil {
		return err
	}
	var msgs []chatstore.Message
	r.mutateSession(func(s *chatstore.Session) {
		msgs = append([]chatstore.Message(nil), s.Messages...)
	})
	model := r.Model
	if r.Cfg != nil {
		model = r.Cfg.ModelDisplayName(r.Prov, r.Model)
	}
	commands.WriteLabeledTranscript(r.Out, msgs, model, r.Cfg != nil && r.Cfg.UsageStatsEnabled())
	tag := checkpoint.FormatCheckpointTag(id.Seq, id.Suffix)
	commands.PrintSystemf(r.Out, "goto %s: transcript truncated (%d messages moved to orphan main).", tag, truncLen)
	return r.persistSession()
}
