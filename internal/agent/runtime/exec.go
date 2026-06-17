package agentruntime

import (
	"context"
	"time"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
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
		RunSubagent:            r.runSubagentFromTool,
		StartResearch:          r.startResearchFromTool,
		ResearchStatus:         r.researchStatusFromTool,
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
	if execToolHook != nil {
		return execToolHook(ctx, inv)
	}
	env := r.toolEnv()
	env.ParentToolCallID = inv.ToolCallID
	return agenttools.Exec(ctx, env, r.Mode, inv)
}

func (r *Runtime) runSubagentFromTool(ctx context.Context, req agenttools.SubagentRequest) (agenttools.SubagentResponse, error) {
	parentChatID := ""
	if r.Session != nil {
		parentChatID = r.Session.ID
	}
	cfg := NestedRunConfig{
		SysPromptPath:    req.SysPromptPath,
		SysPrompt:        req.SysPrompt,
		Task:             req.Task,
		ResumeID:         req.Resume,
		RunInBackground:  req.RunInBackground,
		ReasoningEffort:  req.ReasoningEffort,
		ParentChatID:     parentChatID,
		ParentToolCallID: req.ToolCall.ID,
		ToolCall:         req.ToolCall,
		SpawnTime:        time.Now().UTC(),
		Origin:           chatstore.SubOriginParent,
		ProjectHex:       r.ProjHex,
	}
	res, err := r.runSubagentTool(ctx, cfg)
	if err != nil {
		return agenttools.SubagentResponse{}, err
	}
	return agenttools.SubagentResponse{
		Output:    res.Output,
		SubchatID: res.SubchatID,
		Status:    res.Status,
	}, nil
}

func (r *Runtime) startResearchFromTool(_ context.Context, query, category string) (research.JobRecord, error) {
	return r.startResearchJob(context.Background(), query, category)
}

func (r *Runtime) researchStatusFromTool(jobID string) (research.JobRecord, error) {
	return r.ResearchStatus(jobID)
}
