package agentruntime

import (
	"context"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
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
		CheckpointStageProjAbs:  r.checkpointStageProjAbs,
		CheckpointBeforeProjAbs: r.checkpointBeforeProjAbs,
		CheckpointRecordEdit:    r.checkpointRecordEdit,
		CheckpointCpSeq:         func() int { return r.currentToolCpSeq },
		AllowDeferredTools:      false,
		SwitchModeCountdown:     r.switchModeCountdown,
		ActivateInstructionsFromAbsPath: func(absPath string) {
			r.activateInstructionsFromAbsPath(absPath)
		},
		ActivateInstructionsFromShellCommand: func(command string) {
			r.activateInstructionsFromShellCommand(command)
		},
		MergeInstructionBlock: r.mergeSystemWithInstructions,
		PlanningActive: func() bool {
			return r.Session != nil && r.Session.PlanningActive
		},
		ActivePlanName: func() string {
			if r.Session == nil {
				return ""
			}
			return r.Session.ActivePlanName
		},
		SetPlanningActive: func(planName string) {
			r.mutateSession(func(s *chatstore.Session) {
				s.PlanningActive = planName != ""
				s.ActivePlanName = planName
				if planName == "" {
					s.PlanImplementing = false
				}
			})
		},
		SetPlanImplementing: func(v bool) {
			r.mutateSession(func(s *chatstore.Session) {
				s.PlanImplementing = v
			})
		},
	}
}

func (r *Runtime) execTool(ctx context.Context, inv tooling.Invocation) (any, error) {
	return agenttools.Exec(ctx, r.toolEnv(), r.Mode, inv)
}
