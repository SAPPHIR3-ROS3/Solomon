package agentruntime

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func (r *Runtime) regenerate(ctx context.Context) (turnErr error) {
	if config.NeedsOnboard(r.Cfg) || r.Prov == nil {
		return fmt.Errorf("config not set up; use /onboard")
	}
	if err := r.ensureReplSessionFileLock(); err != nil {
		return err
	}
	if err := r.waitProviderReady(ctx); err != nil {
		return err
	}
	var err error
	r.mutateSession(func(s *chatstore.Session) { err = s.TruncateForRegeneration() })
	if err != nil {
		return err
	}
	if err := r.persistSession(); err != nil {
		return err
	}
	if r.EventSink != nil {
		// The daemon publishes the truncated transcript before assistant deltas.
		r.EventSink.Emit(cievents.Event{"type": "regenerate_start"})
	} else {
		if err := commands.Clear(r.slashDeps(ctx)); err != nil {
			return err
		}
		commands.WriteLabeledTranscript(r.Out, r.Session.Messages, r.Model, true)
	}
	seq := r.Session.Messages[len(r.Session.Messages)-1].CheckpointSeq
	defer func() {
		if err := r.saveTurnFileChanges(seq); err != nil && turnErr == nil {
			turnErr = err
		}
	}()
	return r.runAgentTurns(ctx)
}
