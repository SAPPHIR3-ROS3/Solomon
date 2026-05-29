package tools

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureCreatePlan(name string, planText string) {}

type createPlanArgs struct {
	Name     string `json:"name"`
	PlanText string `json:"planText"`
}

func createPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("createPlan", "Create or overwrite a plan file (markdown) under the project plans directory.", map[string]any{
		"name":     map[string]any{"type": "string", "description": "Plan filename, e.g. feature.md"},
		"planText": map[string]any{"type": "string", "description": "Full markdown body for the plan"},
	}, []string{"name", "planText"})
}

func appendCreatePlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureCreatePlan)
	if err != nil {
		return err
	}
	b.addBlock("createPlan", "Create or overwrite a plan file (markdown) under the project plans directory.", sig)
	return nil
}

func execCreatePlan(env *Env, raw json.RawMessage) (any, error) {
	var a createPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	fn, err := project.NormalizePlanName(a.Name)
	if err != nil {
		return nil, err
	}
	dir, err := chatPlansDir(env.ProjHex)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, fn)
	if err := os.WriteFile(p, []byte(a.PlanText), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"path": p, "ok": true}, nil
}
