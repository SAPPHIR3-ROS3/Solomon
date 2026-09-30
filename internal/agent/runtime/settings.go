package agentruntime

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

// reloadSettings runs between completion requests, in a serialized tool batch.
// Do not mutate the old config: in-flight research/subagent requests retain it.
func (r *Runtime) reloadSettings(ctx context.Context, persisted *config.Root) error {
	if r.Cfg == nil || persisted == nil {
		return fmt.Errorf("runtime config unavailable")
	}
	next := config.ReloadSettings(r.Cfg, persisted)
	backend := r.Backend
	// Cursor Sub retains cfg internally (fast mode); other backends consume
	// preferences from TurnRequest.Cfg on each request.
	if r.Prov != nil && r.Prov.IsCursorSub() {
		var err error
		backend, err = llm.NewCompletionBackend(context.WithoutCancel(ctx), next, r.Prov)
		if err != nil {
			return err
		}
	}
	r.Cfg = next
	r.Backend = backend
	r.CompactionThresholdTokens = config.EffectiveCompactionThresholdTokens(next)
	r.roleBackends.mu.Lock()
	r.roleBackends.byID = nil
	r.roleBackends.mu.Unlock()
	logging.Log(logging.INFO_LOG_LEVEL, "settings reloaded", logging.LogOptions{Params: map[string]any{"compaction_threshold_tokens": r.CompactionThresholdTokens}})
	return nil
}
