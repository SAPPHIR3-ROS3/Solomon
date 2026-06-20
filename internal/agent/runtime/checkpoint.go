package agentruntime

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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
		logging.Log(logging.ERROR_LOG_LEVEL, "checkpoint staging load failed", logging.LogOptions{Params: map[string]any{"dir": dir, "err": err.Error()}})
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
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "checkpoint staging unavailable", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
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
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "checkpoint staging unavailable", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
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
	var truncLen int
	var restoredFromBranch bool
	r.mutateSession(func(s *chatstore.Session) {
		prevLen := len(s.Messages)
		keep, branches, err := checkpoint.ResolveSessionGoto(s, id)
		if err != nil {
			splitErr = err
			return
		}
		restoredFromBranch = len(keep) > prevLen
		s.Messages = keep
		s.Branches = branches
		s.CheckpointBranchSuffix = checkpoint.NextForkSuffix(s, id.Seq)
		chatstore.FinishSessionLoad(s)
		if restoredFromBranch {
			truncLen = len(keep) - prevLen
		} else if len(keep) < prevLen {
			truncLen = prevLen - len(keep)
		}
	})
	if splitErr != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "checkpoint goto failed", logging.LogOptions{Params: map[string]any{"checkpoint": checkpoint.FormatCheckpointTag(id.Seq, id.Suffix), "err": splitErr.Error()}})
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
	if restoredFromBranch {
		commands.PrintSystemf(r.Out, "goto %s: restored branch (%d messages).", tag, truncLen)
	} else {
		commands.PrintSystemf(r.Out, "goto %s: transcript truncated (%d messages moved to another branch).", tag, truncLen)
	}
	return r.persistSession()
}

func (r *Runtime) ApplyRewindCheckpoint(plan *checkpoint.RewindPlan) error {
	if plan == nil || plan.Target == nil {
		return fmt.Errorf("invalid rewind plan")
	}
	id := plan.Target
	store, _ := r.stagingStore()
	if store != nil && r.ProjRoot != "" {
		res, err := store.RestoreToCheckpoint(id.Seq, r.ProjRoot)
		if err != nil {
			commands.PrintSystemf(r.Out, "rewind: file restore warning: %v", err)
		} else if res.FilesWritten > 0 || res.FilesRemoved > 0 {
			commands.PrintSystemf(r.Out, "rewind: restored workspace (%d written, %d removed).", res.FilesWritten, res.FilesRemoved)
		}
		for _, w := range res.Warnings {
			commands.PrintSystemf(r.Out, "rewind: %s", w)
		}
	}
	r.mutateSession(func(s *chatstore.Session) {
		s.Messages = append([]chatstore.Message(nil), plan.Messages...)
		s.Branches = append([]chatstore.BranchSegment(nil), plan.Branches...)
		s.ForkChildCount = checkpoint.PruneForkChildCount(s.ForkChildCount, id.Seq)
		s.CheckpointBranchSuffix = checkpoint.NextForkSuffix(s, id.Seq)
		chatstore.FinishSessionLoad(s)
	})
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
	commands.PrintSystemf(r.Out, "rewind %s: deleted %d message(s) and %d alternate branch(es) (%d message(s)).", tag, plan.DroppedMsgs, plan.RemovedBranches, plan.RemovedBranchMsgs)
	return r.persistSession()
}
