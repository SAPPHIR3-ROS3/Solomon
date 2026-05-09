package tools

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureSearchSkill(query string) {}

type searchSkillArgs struct {
	Query string `json:"query"`
}

func searchSkillOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("searchSkill", "BM25 search over installed skills; returns one best match or an error if nothing passes the quality threshold. Tries descriptions first, then full SKILL.md. The returned score is normalized to [0,1] (best raw BM25 for the query divided by a corpus ceiling). Default minimum is 0.05 (config key skill_search_min_normalized_score; set to 0 to disable).", map[string]any{
		"query": map[string]any{"type": "string", "description": "Search query"},
	}, []string{"query"})
}

func appendSearchSkillDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSearchSkill)
	if err != nil {
		return err
	}
	b.addBlock("searchSkill", "BM25 search; two phases; score normalized [0,1]; min threshold from config (default 0.05).", sig)
	return nil
}

func execSearchSkill(env *Env, raw json.RawMessage) (any, error) {
	var a searchSkillArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	hit, err := skills.SearchBestInstalledSkill(a.Query, env.ProjHex, env.ProjRoot, config.EffectiveSkillSearchMinNorm(env.Cfg))
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
