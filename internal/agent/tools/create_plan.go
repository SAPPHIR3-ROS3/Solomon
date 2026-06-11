package tools

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureCreatePlan(name string, goal string) {}

type createPlanArgs struct {
	Name string `json:"name"`
	Goal string `json:"goal"`
}

func createPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("createPlan", "Create a structured plan file with frontmatter and Goal section under the project plans directory.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename, e.g. feature.md"},
		"goal": map[string]any{"type": "string", "description": "One-sentence goal for the feature or task"},
	}, []string{"name", "goal"})
}

func appendCreatePlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureCreatePlan)
	if err != nil {
		return err
	}
	b.addBlock("createPlan", "Create a structured plan file with frontmatter and Goal section under the project plans directory.", sig)
	return nil
}

func execCreatePlan(env *Env, raw json.RawMessage) (any, error) {
	var a createPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	fn, err := resolvePlanName(env, a.Name)
	if err != nil {
		return nil, err
	}
	dir, err := chatPlansDir(env.ProjHex)
	if err != nil {
		return nil, err
	}
	p, err := plan.ResolvePath(dir, fn)
	if err != nil {
		return nil, err
	}
	git := plan.GitMetaFromRoot(env.ProjRoot)
	meta := plan.NewMeta(git, plan.StatusNotBuilt)
	body := plan.SkeletonBody(a.Goal)
	doc, err := plan.WriteDocument(meta, body)
	if err != nil {
		return nil, err
	}
	if err := writePlanBytes(p, doc); err != nil {
		return nil, err
	}
	pending, _ := plan.CountPending(dir)
	activatePlan(env, fn)
	return map[string]any{"ok": true, "path": p, "pending_plans": pending}, nil
}
