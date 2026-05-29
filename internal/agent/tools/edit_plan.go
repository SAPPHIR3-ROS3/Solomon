package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureEditPlan(name, oldStr, newStr, intent string) {}

type editPlanArgs struct {
	Name   string `json:"name"`
	Old    string `json:"old"`
	New    string `json:"new"`
	Intent string `json:"intent"`
}

func editPlanOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("editPlan", "Replace first occurrence of old segment in plan file.", map[string]any{
		"name":   map[string]any{"type": "string", "description": "Plan filename"},
		"old":    map[string]any{"type": "string", "description": "Exact substring to replace once"},
		"new":    map[string]any{"type": "string", "description": "Replacement text"},
		"intent": map[string]any{"type": "string", "description": "Brief phrase describing the purpose of this plan edit"},
	}, []string{"name", "old", "new", "intent"})
}

func appendEditPlanDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureEditPlan)
	if err != nil {
		return err
	}
	b.addBlock("editPlan", "Replace first occurrence of old segment in plan file. Requires intent (brief purpose).", sig)
	return nil
}

func execEditPlan(env *Env, raw json.RawMessage) (any, error) {
	var a editPlanArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.Intent) == "" {
		return nil, fmt.Errorf("intent must be a non-empty brief phrase")
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
	s := string(b)
	if !strings.Contains(s, a.Old) {
		return map[string]any{"ok": false, "reason": "old not found"}, nil
	}
	s = strings.Replace(s, a.Old, a.New, 1)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "path": p, "intent": a.Intent}, nil
}
