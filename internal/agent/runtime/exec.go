package agentruntime

import (
	"context"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func (r *Runtime) toolEnv() *agenttools.Env {
	return &agenttools.Env{
		ProjHex:                r.ProjHex,
		ProjRoot:               r.ProjRoot,
		Cfg:                    r.Cfg,
		MCP:                    r.MCP,
		RunNested:              r.runNested,
		RunNestedWithSystem:    r.runNestedWithSystem,
		SetMode:                func(m string) { r.Mode = m },
		CurrentMode:            func() string { return r.Mode },
		CheckpointStageProjAbs: r.checkpointStageProjAbs,
	}
}

func (r *Runtime) execTool(ctx context.Context, inv tooling.Invocation) (any, error) {
	return agenttools.Exec(ctx, r.toolEnv(), r.Mode, inv)
}
