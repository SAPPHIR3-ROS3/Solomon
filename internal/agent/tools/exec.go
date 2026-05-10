package tools

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func Exec(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	if env.MCP != nil && env.MCP.HasTool(inv.Name) {
		return env.MCP.CallTool(ctx, inv.Name, inv.Args)
	}
	switch inv.Name {
	case "createPlan", "editPlan", "buildPlan":
		if mode != "plan" {
			err := fmt.Errorf("tool %s only in /plan mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/plan"}})
			return nil, err
		}
	case "shell", "readFile", "editFile", "subagent", "loadSkill", "searchSkill", "fetchWeb", "webSearch":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
	switch inv.Name {
	case "createPlan":
		return execCreatePlan(env, inv.Args)
	case "editPlan":
		return execEditPlan(env, inv.Args)
	case "buildPlan":
		return execBuildPlan(ctx, env, inv.Args)
	case "shell":
		return execShell(ctx, env, inv.Args)
	case "readFile":
		return execReadFile(env, inv.Args)
	case "editFile":
		return execEditFile(env, inv.Args)
	case "subagent":
		return execSubagent(ctx, env, inv.Args)
	case "loadSkill":
		return execLoadSkill(env, inv.Args)
	case "searchSkill":
		return execSearchSkill(env, inv.Args)
	case "fetchWeb":
		return execFetchWeb(ctx, inv.Args)
	case "webSearch":
		return execWebSearch(ctx, env, inv.Args)
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool dispatch", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
}
