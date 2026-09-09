package webfetch

import (
	"context"
	"fmt"
	"strings"
)

const (
	ParallelAdapterName   = "parallel"
	ParallelFetchToolName = "web_fetch"
)

type ParallelFetcher struct {
	caller    MCPToolCaller
	server    string
	fetchTool string
}

func NewParallelFetcher(caller MCPToolCaller, serverName string) (*ParallelFetcher, error) {
	if caller == nil {
		return nil, fmt.Errorf("parallel fetcher: MCP caller is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("parallel fetcher: server name is required")
	}
	return &ParallelFetcher{caller: caller, server: serverName, fetchTool: ParallelFetchToolName}, nil
}

func (f *ParallelFetcher) Fetch(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.caller == nil {
		return Result{}, fmt.Errorf("parallel fetch: MCP caller unavailable")
	}
	pageURL, err := validateFetchURL(req.URL)
	if err != nil {
		return Result{}, fmt.Errorf("parallel: %w", err)
	}
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web fetch: " + pageURL
	}
	args := map[string]any{
		"urls":         []string{pageURL},
		"full_content": true,
	}
	objective := extraString(req.Extras, "objective", "targetContent", "target_content")
	if objective == "" {
		objective = intent
	}
	if len([]rune(objective)) > 200 {
		objective = string([]rune(objective)[:200])
	}
	if objective != "" {
		args["objective"] = objective
	}
	if queries := extraStringSlice(req.Extras, "searchQueries", "search_queries"); len(queries) > 0 {
		if len(queries) > 5 {
			queries = queries[:5]
		}
		args["search_queries"] = queries
	}
	if sessionID := extraString(req.Extras, "sessionID", "session_id"); sessionID != "" {
		args["session_id"] = sessionID
	}
	if modelName := extraString(req.Extras, "modelName", "model_name"); modelName != "" {
		args["model_name"] = modelName
	}
	callCtx, cancel := context.WithTimeout(ctx, fetchTimeout(req))
	defer cancel()
	raw, err := f.caller.CallInternalTool(callCtx, f.server, f.fetchTool, intent, args)
	if err != nil {
		return Result{}, fmt.Errorf("parallel: %w", err)
	}
	result, err := normalizeMCPFetchResponse(raw, ParallelAdapterName, pageURL)
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
