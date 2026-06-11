package tools

import (
	"encoding/json"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureBuildPlan(name string) {}

type buildPlanArgs struct {
	Name string `json:"name"`
}

func buildPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("buildPlan", "Prepare structured implementation brief from a plan (no nested run). Sets plan implementing mode.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename to implement"},
	}, []string{"name"})
}

func appendBuildPlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureBuildPlan)
	if err != nil {
		return err
	}
	b.addBlock("buildPlan", "Prepare structured implementation brief from a plan (no nested run). Sets plan implementing mode.", sig)
	return nil
}

func execBuildPlan(env *Env, raw json.RawMessage) (any, error) {
	var a buildPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p, err := planPath(env, a.Name)
	if err != nil {
		return nil, err
	}
	meta, sec, _, err := plan.ReadFile(p)
	if err != nil {
		return nil, err
	}
	open := plan.OpenTodos(sec.Todo.Checklist)
	if len(open) == 0 {
		return map[string]any{"ok": false, "reason": "no open todos"}, nil
	}
	if meta.Status == plan.StatusBuilt {
		return map[string]any{"ok": false, "reason": "plan already built"}, nil
	}
	var remaining []map[string]string
	for _, it := range open {
		remaining = append(remaining, map[string]string{"sha": it.SHA, "todo": it.Text})
	}
	if env.SetPlanImplementing != nil {
		env.SetPlanImplementing(true)
	}
	fn, _ := resolvePlanName(env, a.Name)
	activatePlan(env, fn)
	return map[string]any{
		"ok":               true,
		"goal":             sec.Goal,
		"design_excerpt":   sec.Design,
		"rules":            sec.Todo.Rules,
		"mermaid":          strings.TrimSpace(sec.Todo.Mermaid),
		"remaining_todos":  remaining,
		"status":           meta.Status,
	}, nil
}
