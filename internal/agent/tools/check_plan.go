package tools

import (
	"encoding/json"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureCheckPlan(name string, full bool) {}

type checkPlanArgs struct {
	Name string `json:"name"`
	Full *bool  `json:"full"`
}

func checkPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("checkPlan", "Inspect plan status and remaining todos, or return full plan body.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename or path"},
		"full": map[string]any{"type": "boolean", "description": "If true, return entire plan markdown body"},
	}, []string{"name"})
}

func deletePlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("deletePlan", "Delete a plan file from the project plans directory.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename or path"},
	}, []string{"name"})
}

func appendCheckPlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureCheckPlan)
	if err != nil {
		return err
	}
	b.addBlock("checkPlan", "Inspect plan status and remaining todos, or return full plan body.", sig)
	return nil
}

func appendDeletePlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureDeletePlan)
	if err != nil {
		return err
	}
	b.addBlock("deletePlan", "Delete a plan file from the project plans directory.", sig)
	return nil
}

func signatureDeletePlan(name string) {}

func execCheckPlan(env *Env, raw json.RawMessage) (any, error) {
	var a checkPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p, err := planPath(env, a.Name)
	if err != nil {
		return nil, err
	}
	meta, sec, b, err := plan.ReadFile(p)
	if err != nil {
		return nil, err
	}
	full := a.Full != nil && *a.Full
	if full {
		_, body, err := plan.ParseDocument(b)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"ok":     true,
			"path":   p,
			"status": meta.Status,
			"body":   string(body),
		}, nil
	}
	open := plan.OpenTodos(sec.Todo.Checklist)
	var remaining []map[string]string
	for _, it := range open {
		remaining = append(remaining, map[string]string{"sha": it.SHA, "todo": it.Text})
	}
	return map[string]any{
		"ok":              true,
		"path":            p,
		"status":          meta.Status,
		"remaining_todos": remaining,
	}, nil
}

func execDeletePlan(env *Env, raw json.RawMessage) (any, error) {
	var a checkPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p, err := planPath(env, a.Name)
	if err != nil {
		return nil, err
	}
	fn, _ := resolvePlanName(env, a.Name)
	if err := os.Remove(p); err != nil {
		return nil, err
	}
	if env.ActivePlanName != nil && env.ActivePlanName() == fn {
		if env.SetPlanningActive != nil {
			env.SetPlanningActive("")
		}
		if env.SetPlanImplementing != nil {
			env.SetPlanImplementing(false)
		}
	}
	return map[string]any{"ok": true, "path": p}, nil
}
