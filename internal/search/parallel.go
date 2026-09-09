package search

import (
	"context"
	"fmt"
	"strings"
)

const (
	ParallelAdapterName    = "parallel"
	ParallelSearchToolName = "web_search"
	ParallelFetchToolName  = "web_fetch"
)

// ParallelAdapter translates Solomon's neutral request into the current
// Parallel Search MCP contract. The MCP tool accepts an objective and one or
// more search queries; max-results and advanced settings are connection-level
// settings on the public MCP endpoint, so the adapter trims the returned list
// locally instead of sending unsupported per-call arguments.
type ParallelAdapter struct {
	caller     MCPToolCaller
	server     string
	searchTool string
}

func NewParallelAdapter(caller MCPToolCaller, serverName string) (*ParallelAdapter, error) {
	if caller == nil {
		return nil, fmt.Errorf("parallel adapter: MCP caller is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("parallel adapter: server name is required")
	}
	return &ParallelAdapter{caller: caller, server: serverName, searchTool: ParallelSearchToolName}, nil
}

func (a *ParallelAdapter) Search(ctx context.Context, req Request) (Response, error) {
	if a == nil || a.caller == nil {
		return Response{}, fmt.Errorf("parallel: MCP caller unavailable")
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return Response{}, fmt.Errorf("parallel: empty query")
	}
	max := req.MaxResults
	if max <= 0 {
		max = defaultSearchResults
	}

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web search: " + query
	}
	args := map[string]any{
		"objective":      query,
		"search_queries": parallelSearchQueries(query, req.Extras),
	}
	if sessionID := extraString(req.Extras, "sessionID", "session_id"); sessionID != "" {
		args["session_id"] = sessionID
	}
	if modelName := extraString(req.Extras, "modelName", "model_name"); modelName != "" {
		args["model_name"] = modelName
	}
	if objective := extraString(req.Extras, "objective"); objective != "" {
		args["objective"] = objective
	}

	raw, err := a.caller.CallInternalTool(ctx, a.server, a.searchTool, intent, args)
	if err != nil {
		return Response{}, fmt.Errorf("parallel: %w", err)
	}
	response, err := normalizeMCPResponse(raw, ParallelAdapterName, max)
	if err != nil {
		return Response{}, err
	}
	return response, nil
}

func parallelSearchQueries(query string, extras map[string]any) []string {
	if queries := extraStringSlice(extras, "searchQueries", "search_queries"); len(queries) > 0 {
		if len(queries) > 5 {
			queries = queries[:5]
		}
		return queries
	}
	return []string{parallelKeywordQuery(query)}
}

// parallelKeywordQuery keeps the full natural-language request in objective
// while supplying the MCP tool with a compact query in its recommended shape.
func parallelKeywordQuery(query string) string {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) <= 6 {
		return strings.Join(words, " ")
	}
	selected := make([]string, 0, 6)
	for _, word := range words {
		clean := strings.Trim(word, ".,;:!?()[]{}\"")
		if clean == "" || parallelStopWords[strings.ToLower(clean)] {
			continue
		}
		selected = append(selected, word)
		if len(selected) == 6 {
			break
		}
	}
	if len(selected) >= 3 {
		return strings.Join(selected, " ")
	}
	return strings.Join(words, " ")
}

var parallelStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "about": true,
	"for": true, "from": true, "how": true, "in": true, "is": true,
	"of": true, "on": true, "or": true, "the": true, "to": true,
	"what": true, "when": true, "where": true, "which": true,
	"who": true, "with": true,
}

func extraString(extras map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := extras[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func extraStringSlice(extras map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := extras[key]
		if !ok || value == nil {
			continue
		}
		var out []string
		switch items := value.(type) {
		case []string:
			out = append(out, items...)
		case []any:
			for _, item := range items {
				if text, ok := item.(string); ok {
					out = append(out, text)
				}
			}
		}
		filtered := out[:0]
		for _, item := range out {
			if item = strings.TrimSpace(item); item != "" {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}
	return nil
}
