package agent

import (
	"context"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
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
	if r == nil || r.MCP == nil {
		return nil
	}
	return r.MCP.Close()
}

func (r *Runtime) toolParams() ([]openai.ChatCompletionToolUnionParam, error) {
	tools, err := NativeToolParams(r.Mode)
	if err != nil {
		return nil, err
	}
	if r.MCP != nil {
		tools = append(tools, r.MCP.OpenAITools()...)
	}
	return tools, nil
}
