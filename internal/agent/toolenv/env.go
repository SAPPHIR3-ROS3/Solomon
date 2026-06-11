package toolenv

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
)

type Env struct {
	ProjHex                string
	ProjRoot               string
	Cfg                    *config.Root
	MCP                    *solomonmcp.Manager
	RunNested              func(ctx context.Context, body string) (string, error)
	RunNestedWithSystem    func(ctx context.Context, sys, task string) (string, error)
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

	PlanningActive      func() bool
	ActivePlanName      func() string
	SetPlanningActive   func(planName string)
	SetPlanImplementing func(bool)
}
