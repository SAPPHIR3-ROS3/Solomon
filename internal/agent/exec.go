package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"solomon/internal/chatstore"
	"solomon/internal/project"
	"solomon/internal/tooling"
)

func (r *Runtime) execTool(ctx context.Context, inv tooling.Invocation) (any, error) {
	switch inv.Name {
	case "createPlan", "editPlan", "buildPlan":
		if r.Mode != "plan" {
			return nil, fmt.Errorf("tool %s only in /plan mode", inv.Name)
		}
	case "shell", "readFile", "editFile", "subagent":
		if r.Mode != "build" {
			return nil, fmt.Errorf("tool %s only in /build mode", inv.Name)
		}
	default:
		return nil, fmt.Errorf("unknown tool %q", inv.Name)
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
	default:
		return nil, fmt.Errorf("unknown tool %q", inv.Name)
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
