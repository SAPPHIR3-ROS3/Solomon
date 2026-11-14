package agentruntime

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type subagentDepthKey struct{}
type subagentCallerKey struct{}

func subagentConfigDepth(ctx context.Context, cfg NestedRunConfig) NestedRunConfig {
	if cfg.Depth <= 0 {
		parent, _ := ctx.Value(subagentDepthKey{}).(int)
		cfg.Depth = parent + 1
	}
	if cfg.ParentSubchatID == "" {
		cfg.ParentSubchatID, _ = ctx.Value(subagentCallerKey{}).(string)
	}
	return cfg
}

func subagentSessionDepth(session *chatstore.SubSession) int {
	if session.Depth > 0 {
		return session.Depth
	}
	return 1
}

func (r *Runtime) validateSubagentDepth(depth int) error {
	limit := config.SubagentDepthLimit(r.Cfg)
	if depth > limit {
		return fmt.Errorf("subagent recursion limit reached: requested depth %d, maximum %d; complete the task without spawning another subagent", depth, limit)
	}
	return nil
}
