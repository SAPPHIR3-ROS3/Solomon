package agentruntime

import (
	"context"
	"os"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooloutput"
	"github.com/openai/openai-go/v2"
)

func (r *Runtime) InitMCP(ctx context.Context) {
	mgr, err := solomonmcp.Start(ctx, os.Stderr)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP disabled", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return
	}
	r.MCP = mgr
}

func (r *Runtime) Close() error {
	var errMCP error
	if r != nil && r.MCP != nil {
		errMCP = r.MCP.Close()
	}
	if r != nil {
		pid := tooloutput.CurrentPID()
		projHex := r.ProjHex
		if err := tooloutput.Shutdown(pid, projHex, r.ToolOut); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "tool output shutdown failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}
	return errMCP
}

func (r *Runtime) toolParams() ([]openai.ChatCompletionToolUnionParam, error) {
	tools, err := agenttools.NativeToolParams(r.Mode)
	if err != nil {
		return nil, err
	}
	if r.MCP != nil {
		tools = append(tools, r.MCP.OpenAITools()...)
	}
	return tools, nil
}
