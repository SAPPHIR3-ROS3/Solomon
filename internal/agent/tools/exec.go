package tools

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func Exec(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	return resolveToolInvocation(ctx, env, mode, inv)
}

func resolveToolInvocation(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	if isInternalToolName(inv.Name) {
		return dispatchInternal(ctx, env, mode, inv)
	}
	if isSkillToolName(inv.Name) || (env.MCP != nil && env.MCP.HasTool(inv.Name)) {
		return dispatchExternal(ctx, env, mode, inv)
	}
	err := fmt.Errorf("unknown tool %q", inv.Name)
	logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
	return nil, err
}

func isInternalToolName(name string) bool {
	switch name {
	case "docsRetrieval",
		"createPlan", "editPlan", "buildPlan",
		"shell", "readFile", "editFile", "find", "subagent", "fetchWeb", "webSearch":
		return true
	default:
		return false
	}
}

func isSkillToolName(name string) bool {
	return name == "loadSkill" || name == "searchSkill"
}

func dispatchInternal(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	switch inv.Name {
	case "docsRetrieval":
		return execDocsRetrieval(env, inv.Args)
	case "createPlan":
		if mode != "plan" {
			err := fmt.Errorf("tool %s only in /plan mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/plan"}})
			return nil, err
		}
		return execCreatePlan(env, inv.Args)
	case "editPlan":
		if mode != "plan" {
			err := fmt.Errorf("tool %s only in /plan mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/plan"}})
			return nil, err
		}
		return execEditPlan(env, inv.Args)
	case "buildPlan":
		if mode != "plan" {
			err := fmt.Errorf("tool %s only in /plan mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/plan"}})
			return nil, err
		}
		return execBuildPlan(ctx, env, inv.Args)
	case "shell":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execShell(ctx, env, inv.Args)
	case "readFile":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execReadFile(env, inv.Args)
	case "find":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execFind(ctx, env, inv.Args)
	case "editFile":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execEditFile(env, inv.Args)
	case "subagent":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execSubagent(ctx, env, inv.Args)
	case "fetchWeb":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execFetchWeb(ctx, inv.Args)
	case "webSearch":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execWebSearch(ctx, env, inv.Args)
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
}

func dispatchExternal(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	if isSkillToolName(inv.Name) {
		return dispatchSkill(env, mode, inv)
	}
	if env.MCP != nil && env.MCP.HasTool(inv.Name) {
		return env.MCP.CallTool(ctx, inv.Name, inv.Args)
	}
	err := fmt.Errorf("unknown tool %q", inv.Name)
	logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
	return nil, err
}

func dispatchSkill(env *Env, mode string, inv tooling.Invocation) (any, error) {
	switch inv.Name {
	case "loadSkill":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execLoadSkill(env, inv.Args)
	case "searchSkill":
		if mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": mode, "need": "/build"}})
			return nil, err
		}
		return execSearchSkill(env, inv.Args)
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
}
