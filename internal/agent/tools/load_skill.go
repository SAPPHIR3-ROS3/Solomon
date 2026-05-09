package tools

import (
	"encoding/json"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureLoadSkill(name string) {}

type loadSkillArgs struct {
	Name string `json:"name"`
}

func loadSkillOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("loadSkill", "Load installed agent skill body (markdown body only). Name is the display name or slash token without leading slash.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Skill display name or slash command token (e.g. my-skill)"},
	}, []string{"name"})
}

func appendLoadSkillDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureLoadSkill)
	if err != nil {
		return err
	}
	b.addBlock("loadSkill", "Load installed agent skill body (markdown body only).", sig)
	return nil
}

func execLoadSkill(env *Env, raw json.RawMessage) (any, error) {
	var a loadSkillArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	e, slash, err := skills.ResolveSkillForLoad(a.Name, env.ProjHex, env.ProjRoot)
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
