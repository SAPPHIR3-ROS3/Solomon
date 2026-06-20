package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureDeepResearch(query string, category string) {}

type deepResearchArgs struct {
	Query    string `json:"query"`
	Category string `json:"category,omitempty"`
}

type deepResearchResult struct {
	OK     bool   `json:"ok"`
	JobID  string `json:"jobId,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

func deepResearchOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("deepResearch", "Start a background deep research job: multi-step web research with structured HTML report (includes TL;DR). Returns immediately with jobId while research runs.", map[string]any{
		"query": map[string]any{"type": "string", "description": "Research question or topic"},
		"category": map[string]any{
			"type":        "string",
			"description": "Optional report format: product, comparison, howto, factcheck",
		},
	}, []string{"query"})
}

func appendDeepResearchDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureDeepResearch)
	if err != nil {
		return err
	}
	b.addBlock("deepResearch", "Start a background deep research job with HTML report output.", sig)
	return nil
}

func execDeepResearch(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a deepResearchArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	a.Query = strings.TrimSpace(a.Query)
	if a.Query == "" {
		return nil, fmt.Errorf("deepResearch: empty query")
	}
	if env.StartResearch == nil {
		return nil, fmt.Errorf("deepResearch runtime unavailable")
	}
	rec, err := env.StartResearch(ctx, a.Query, strings.TrimSpace(a.Category))
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "deepResearch start failed", logging.LogOptions{Params: map[string]any{"query": a.Query, "err": err.Error()}})
		return deepResearchResult{OK: false, Error: err.Error()}, nil
	}
	logging.Log(logging.INFO_LOG_LEVEL, "deepResearch job started", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "title": rec.Title}})
	return deepResearchResult{
		OK:     true,
		JobID:  rec.ID,
		Title:  rec.Title,
		Status: rec.Status,
	}, nil
}
