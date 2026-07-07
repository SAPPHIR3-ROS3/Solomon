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
	if isSkillToolName(inv.Name) || mcpToolAllowed(env, inv.Name) {
		return dispatchExternal(ctx, env, mode, inv)
	}
	err := fmt.Errorf("unknown tool %q", inv.Name)
	logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
	return nil, err
}

func isInternalToolName(name string) bool {
	switch name {
	case "docsRetrieval",
		"createPlan", "editPlan", "buildPlan", "addTodo", "todoList", "checkTodo", "removeTodo", "checkPlan", "deletePlan",
		"shell", "readFile", "editFile", "find", "listDir", "tree", "subagent", "fetchWeb", "webSearch",
		"deepResearch", "researchStatus",
		"searchTools", "orchestrate", "switchMode", "listSubAgents":
		return true
	default:
		return false
	}
}

func isSkillToolName(name string) bool {
	return name == "loadSkill" || name == "searchSkill"
}

func modeAllowed(env *Env, mode, tool string) bool {
	if IsUniversalTool(tool) {
		return true
	}
	if env != nil && env.AllowDeferredTools {
		switch tool {
		case "searchTools", "orchestrate", "switchMode", "subagent":
			return false
		default:
			return isInternalToolName(tool) || isSkillToolName(tool)
		}
	}
	m := normalizeMode(mode)
	switch m {
	case "agent":
		switch tool {
		case "searchTools", "orchestrate", "switchMode", "loadSkill", "searchSkill", "subagent", "listSubAgents":
			return true
		default:
			if isPlanTool(tool) {
				return planAllowed(env, tool)
			}
			if mcpToolAllowed(env, tool) {
				return true
			}
			return false
		}
	case "chat":
		switch tool {
		case "fetchWeb", "webSearch", "switchMode", "deepResearch", "researchStatus":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func rejectMode(tool, mode string) error {
	err := fmt.Errorf("tool %s not available in %s mode", tool, normalizeMode(mode))
	logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": tool, "mode": mode}})
	return err
}

func dispatchInternal(ctx context.Context, env *Env, mode string, inv tooling.Invocation) (any, error) {
	if !modeAllowed(env, mode, inv.Name) {
		return nil, rejectMode(inv.Name, mode)
	}
	switch inv.Name {
	case "docsRetrieval":
		return execDocsRetrieval(env, inv.Args)
	case "createPlan":
		return execCreatePlan(env, inv.Args)
	case "editPlan":
		return execEditPlan(env, inv.Args)
	case "buildPlan":
		return execBuildPlan(env, inv.Args)
	case "addTodo":
		return execAddTodo(env, inv.Args)
	case "todoList":
		return execTodoList(env, inv.Args)
	case "checkTodo":
		return execCheckTodo(env, inv.Args)
	case "removeTodo":
		return execRemoveTodo(env, inv.Args)
	case "checkPlan":
		return execCheckPlan(env, inv.Args)
	case "deletePlan":
		return execDeletePlan(env, inv.Args)
	case "shell":
		return execShell(ctx, env, inv.Args)
	case "readFile":
		return execReadFile(env, inv.Args)
	case "find":
		return execFind(ctx, env, inv.Args)
	case "listDir":
		return execListDir(env, inv.Args)
	case "tree":
		return execTree(env, inv.Args)
	case "editFile":
		return execEditFile(env, inv.Args)
	case "subagent":
		return execSubagent(ctx, env, inv.Args)
	case "fetchWeb":
		return execFetchWeb(ctx, env, inv.Args)
	case "webSearch":
		return execWebSearch(ctx, env, inv.Args)
	case "deepResearch":
		return execDeepResearch(ctx, env, inv.Args)
	case "researchStatus":
		return execResearchStatus(env, inv.Args)
	case "searchTools":
		return execSearchTools(env, inv.Args)
	case "orchestrate":
		return execOrchestrate(ctx, env, inv.Args)
	case "switchMode":
		return execSwitchMode(ctx, env, inv.Args)
	case "listSubAgents":
		return execListSubAgents(env, inv.Args)
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
		if !modeAllowed(env, mode, inv.Name) && !(env != nil && env.AllowDeferredTools) {
			return nil, rejectMode(inv.Name, mode)
		}
		return env.MCP.CallTool(ctx, inv.Name, inv.Args)
	}
	err := fmt.Errorf("unknown tool %q", inv.Name)
	logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
	return nil, err
}

func dispatchSkill(env *Env, mode string, inv tooling.Invocation) (any, error) {
	if !modeAllowed(env, mode, inv.Name) {
		return nil, rejectMode(inv.Name, mode)
	}
	switch inv.Name {
	case "loadSkill":
		return execLoadSkill(env, inv.Args)
	case "searchSkill":
		return execSearchSkill(env, inv.Args)
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
}
