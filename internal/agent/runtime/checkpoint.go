package agentruntime

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
)

func (r *Runtime) stagingStore() (*staging.Store, error) {
	if r == nil || r.Session == nil || r.ProjHex == "" || r.Session.ID == "" {
		return nil, nil
	}
	if r.stagingCache != nil && r.stagingCacheSession == r.Session.ID {
		return r.stagingCache, nil
	}
	dir, err := staging.SessionDir(r.ProjHex, r.Session.ID)
	if err != nil {
		return nil, err
	}
	store, err := staging.Load(dir)
	if err != nil {
		return nil, err
	}
	r.stagingCache = store
	r.stagingCacheSession = r.Session.ID
	return store, nil
}

func (r *Runtime) checkpointStageProjAbs(absResolved string) {
	if checkpoint.SkipStagingIfRunningExecutable(absResolved) {
		return
	}
}

func (r *Runtime) checkpointBeforeProjAbs(absResolved string) {
	if checkpoint.SkipStagingIfRunningExecutable(absResolved) {
		return
	}
	store, err := r.stagingStore()
	if err != nil || store == nil {
		return
	}
	_ = store.RecordBefore(absResolved)
}

func (r *Runtime) checkpointRecordEdit(kind, absPath, renameTo string, content []byte) {
	if checkpoint.SkipStagingIfRunningExecutable(absPath) {
		return
	}
	store, err := r.stagingStore()
	if err != nil || store == nil {
		return
	}
	_ = store.RecordOp(r.currentToolCpSeq, kind, absPath, renameTo, content)
}

func (r *Runtime) ApplyGotoCheckpoint(id *checkpoint.FullCheckpointID) error {
	store, _ := r.stagingStore()
	if store != nil && r.ProjRoot != "" {
		res, err := store.RestoreToCheckpoint(id.Seq, r.ProjRoot)
		if err != nil {
			commands.PrintSystemf(r.Out, "goto: file restore warning: %v", err)
		} else if res.FilesWritten > 0 || res.FilesRemoved > 0 {
			commands.PrintSystemf(r.Out, "goto: restored workspace (%d written, %d removed).", res.FilesWritten, res.FilesRemoved)
		}
		for _, w := range res.Warnings {
			commands.PrintSystemf(r.Out, "goto: %s", w)
		}
	}
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
