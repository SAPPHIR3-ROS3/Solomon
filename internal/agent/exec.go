package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func (r *Runtime) execTool(ctx context.Context, inv tooling.Invocation) (any, error) {
	if r.MCP != nil && r.MCP.HasTool(inv.Name) {
		return r.MCP.CallTool(ctx, inv.Name, inv.Args)
	}
	switch inv.Name {
	case "createPlan", "editPlan", "buildPlan":
		if r.Mode != "plan" {
			err := fmt.Errorf("tool %s only in /plan mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": r.Mode, "need": "/plan"}})
			return nil, err
		}
	case "shell", "readFile", "editFile", "subagent", "loadSkill", "searchSkill":
		if r.Mode != "build" {
			err := fmt.Errorf("tool %s only in /build mode", inv.Name)
			logging.Log(logging.WARNING_LOG_LEVEL, "tool rejected: wrong session mode", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "mode": r.Mode, "need": "/build"}})
			return nil, err
		}
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
	switch inv.Name {
	case "createPlan":
		return r.toolCreatePlan(inv.Args)
	case "editPlan":
		return r.toolEditPlan(inv.Args)
	case "buildPlan":
		return r.toolBuildPlan(ctx, inv.Args)
	case "shell":
		return r.toolShell(ctx, inv.Args)
	case "readFile":
		return r.toolReadFile(inv.Args)
	case "editFile":
		return r.toolEditFile(inv.Args)
	case "subagent":
		return r.toolSubagent(ctx, inv.Args)
	case "loadSkill":
		return r.toolLoadSkill(inv.Args)
	case "searchSkill":
		return r.toolSearchSkill(inv.Args)
	default:
		err := fmt.Errorf("unknown tool %q", inv.Name)
		logging.Log(logging.WARNING_LOG_LEVEL, "unknown tool dispatch", logging.LogOptions{Params: map[string]any{"tool": inv.Name}})
		return nil, err
	}
}

type createPlanArgs struct {
	Name     string `json:"name"`
	PlanText string `json:"planText"`
}

func (r *Runtime) toolCreatePlan(raw json.RawMessage) (any, error) {
	var a createPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	fn, err := project.NormalizePlanName(a.Name)
	if err != nil {
		return nil, err
	}
	dir, err := chatPlansDir(r.ProjHex)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, fn)
	if err := os.WriteFile(p, []byte(a.PlanText), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"path": p, "ok": true}, nil
}

type editPlanArgs struct {
	Name string `json:"name"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

func (r *Runtime) toolEditPlan(raw json.RawMessage) (any, error) {
	var a editPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	fn, err := project.NormalizePlanName(a.Name)
	if err != nil {
		return nil, err
	}
	dir, err := chatPlansDir(r.ProjHex)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, fn)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	s := string(b)
	if !strings.Contains(s, a.Old) {
		return map[string]any{"ok": false, "reason": "old not found"}, nil
	}
	s = strings.Replace(s, a.Old, a.New, 1)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "path": p}, nil
}

type buildPlanArgs struct {
	Name string `json:"name"`
}

func (r *Runtime) toolBuildPlan(ctx context.Context, raw json.RawMessage) (any, error) {
	var a buildPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	fn, err := project.NormalizePlanName(a.Name)
	if err != nil {
		return nil, err
	}
	dir, err := chatPlansDir(r.ProjHex)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, fn)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	body := string(b)
	r.Mode = "build"
	out, err := r.runNested(ctx, body)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "summary": out}, nil
}

func chatPlansDir(projectHex string) (string, error) {
	return chatstore.PlansDir(projectHex)
}

type loadSkillArgs struct {
	Name string `json:"name"`
}

func (r *Runtime) toolLoadSkill(raw json.RawMessage) (any, error) {
	var a loadSkillArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	e, slash, err := skills.ResolveSkillForLoad(a.Name, r.ProjHex, r.ProjRoot)
	if err != nil {
		return nil, err
	}
	body, err := skills.SkillMarkdownBody(e.SkillMdPath)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":  strings.TrimSpace(e.Name),
		"slash": slash,
		"body":  body,
	}, nil
}

type searchSkillArgs struct {
	Query string `json:"query"`
}

func (r *Runtime) toolSearchSkill(raw json.RawMessage) (any, error) {
	var a searchSkillArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	hit, err := skills.SearchBestInstalledSkill(a.Query, r.ProjHex, r.ProjRoot, config.EffectiveSkillSearchMinNorm(r.Cfg))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":        hit.Name,
		"slash":       hit.Slash,
		"description": hit.Description,
		"score":       hit.Score,
	}, nil
}
