package search

import (
	"context"
	"fmt"
	"strings"
)

const (
	ExaAdapterName       = "exa"
	ExaSearchToolName    = "web_search_exa"
	ExaFetchToolName     = "web_fetch_exa"
	defaultSearchResults = 10
)

// ExaAdapter translates Solomon's neutral search request into Exa's hosted
// MCP search tool. The hosted tool intentionally exposes only query and
// numResults, so provider-specific search settings are not guessed or sent as
// unsupported MCP arguments.
type ExaAdapter struct {
	caller     MCPToolCaller
	server     string
	searchTool string
}

func NewExaAdapter(caller MCPToolCaller, serverName string) (*ExaAdapter, error) {
	if caller == nil {
		return nil, fmt.Errorf("exa adapter: MCP caller is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("exa adapter: server name is required")
	}
	return &ExaAdapter{caller: caller, server: serverName, searchTool: ExaSearchToolName}, nil
}

func (a *ExaAdapter) Search(ctx context.Context, req Request) (Response, error) {
	if a == nil || a.caller == nil {
		return Response{}, fmt.Errorf("exa: MCP caller unavailable")
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return Response{}, fmt.Errorf("exa: empty query")
	}
	max := req.MaxResults
	if max <= 0 {
		max = defaultSearchResults
	}

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web search: " + query
	}
	raw, err := a.caller.CallInternalTool(ctx, a.server, a.searchTool, intent, map[string]any{
		"query":      query,
		"numResults": max,
	})
	if err != nil {
		return Response{}, fmt.Errorf("exa: %w", err)
	}
	response, err := normalizeMCPResponse(raw, ExaAdapterName, max)
	if err != nil {
		return Response{}, err
	}
	return response, nil
}
