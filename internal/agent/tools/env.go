package tools

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
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
}
