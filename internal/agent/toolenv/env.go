package toolenv

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
)

type SubagentRequest struct {
	SysPromptPath   string
	SysPrompt       string
	Task            string
	Resume          string
	RunInBackground bool
	ReasoningEffort string
	RoleProvider    string
	RoleModel       string
	ToolCall        chatstore.ToolCall
}

type SubagentResponse struct {
	Output    string
	SubchatID string
	Status    string
}

type Env struct {
	ProjHex                string
	ProjRoot               string
	Cfg                    *config.Root
	MCP                    *solomonmcp.Manager
	RunNested              func(ctx context.Context, body string) (string, error)
	RunNestedWithSystem    func(ctx context.Context, sys, task string) (string, error)
	RunSubagent            func(ctx context.Context, req SubagentRequest) (SubagentResponse, error)
	ParentToolCallID       string
	SetMode                func(string)
	CurrentMode            func() string
	CheckpointStageProjAbs func(path string)
	CheckpointBeforeProjAbs func(path string)
	CheckpointRecordEdit    func(kind, path, renameTo string, content []byte)
	CheckpointCpSeq         func() int
	AllowDeferredTools      bool
	SwitchModeCountdown     func(ctx context.Context, target string) (cancelled bool, err error)
	ActivateInstructionsFromAbsPath func(absPath string)
	ActivateInstructionsFromShellCommand func(command string)
	MergeInstructionBlock func(customSys string) (string, error)

	StartResearch   func(ctx context.Context, query, category string) (research.JobRecord, error)
	ResearchStatus  func(jobID string) (research.JobRecord, error)

	PlanningActive      func() bool
	ActivePlanName      func() string
	SetPlanningActive   func(planName string)
	SetPlanImplementing func(bool)
}
