package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureBuildPlan(name string) {}

type buildPlanArgs struct {
	Name string `json:"name"`
}

func buildPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("buildPlan", "Switch to BUILD mode and run an implementation session for the named plan.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename to implement"},
	}, []string{"name"})
}

func appendBuildPlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureBuildPlan)
	if err != nil {
		return err
	}
	b.addBlock("buildPlan", "Switch to BUILD mode and run an implementation session for the named plan.", sig)
	return nil
}

func execBuildPlan(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a buildPlanArgs
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
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	body := string(b)
	env.SetMode("build")
	out, err := env.RunNested(ctx, body)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "summary": out}, nil
}
