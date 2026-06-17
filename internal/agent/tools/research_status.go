package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureResearchStatus(jobID string) {}

type researchStatusArgs struct {
	JobID string `json:"jobId"`
}

func researchStatusOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("researchStatus", "Get status of a deep research job by jobId (phase, round, sources, report path when done).", map[string]any{
		"jobId": map[string]any{"type": "string", "description": "Research job ID from deepResearch"},
	}, []string{"jobId"})
}

func appendResearchStatusDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureResearchStatus)
	if err != nil {
		return err
	}
	b.addBlock("researchStatus", "Get status of a deep research job by jobId.", sig)
	return nil
}

func execResearchStatus(env *Env, raw json.RawMessage) (any, error) {
	var a researchStatusArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	a.JobID = strings.TrimSpace(a.JobID)
	if a.JobID == "" {
		return nil, fmt.Errorf("researchStatus: empty jobId")
	}
	if env.ResearchStatus == nil {
		return nil, fmt.Errorf("researchStatus runtime unavailable")
	}
	rec, err := env.ResearchStatus(a.JobID)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"jobId":     rec.ID,
		"title":     rec.Title,
		"status":    rec.Status,
		"phase":     rec.Phase,
		"round":     rec.Round,
		"maxRounds": rec.MaxRounds,
		"slug":      rec.Slug,
	}
	if rec.HTMLPath != "" {
		out["htmlPath"] = rec.HTMLPath
	}
	if rec.Stats.URLs > 0 {
		out["urls"] = rec.Stats.URLs
	}
	if rec.Stats.Findings > 0 {
		out["findings"] = rec.Stats.Findings
	}
	if rec.Stats.DurationSecs > 0 {
		out["durationSecs"] = rec.Stats.DurationSecs
	}
	if rec.Error != "" {
		out["error"] = rec.Error
	}
	if rec.Status == research.StatusDone {
		out["ok"] = true
	}
	return out, nil
}
