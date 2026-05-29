package agentruntime

import (
	"context"
	"fmt"
	"os"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
	"github.com/openai/openai-go/v2"
)

func (r *Runtime) InitMCP(ctx context.Context) {
	mgr, err := solomonmcp.StartLazy(os.Stderr)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP disabled", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return
	}
	r.MCP = mgr
	go r.connectMCPBackground(ctx, mgr)
}

func (r *Runtime) connectMCPBackground(ctx context.Context, mgr *solomonmcp.Manager) {
	configured, err := solomonmcp.ConfiguredServerCount()
	if err != nil || configured == 0 {
		_, _, _ = mgr.Connect(ctx)
		return
	}
	servers, tools, err := mgr.Connect(ctx)
	if r.Out == nil {
		return
	}
	if err != nil {
		termcolor.WriteSystem(r.Out, fmt.Sprintf("MCP: connect failed (%v)", err))
		return
	}
	if servers == 0 {
		termcolor.WriteSystem(r.Out, fmt.Sprintf("MCP: 0/%d servers connected", configured))
		return
	}
	termcolor.WriteSystem(r.Out, fmt.Sprintf("MCP: %d/%d servers, %d tool(s) ready", servers, configured, tools))
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
	if r.MCP != nil && r.MCP.IsReady() {
		tools = append(tools, r.MCP.OpenAITools()...)
	}
	return tools, nil
}
