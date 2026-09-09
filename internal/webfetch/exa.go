package webfetch

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

const (
	ExaAdapterName       = "exa"
	ExaFetchToolName     = "web_fetch_exa"
	defaultExaCharacters = 50000
)

type ExaFetcher struct {
	caller    MCPToolCaller
	server    string
	fetchTool string
}

func NewExaFetcher(caller MCPToolCaller, serverName string) (*ExaFetcher, error) {
	if caller == nil {
		return nil, fmt.Errorf("exa fetcher: MCP caller is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("exa fetcher: server name is required")
	}
	return &ExaFetcher{caller: caller, server: serverName, fetchTool: ExaFetchToolName}, nil
}

func (f *ExaFetcher) Fetch(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.caller == nil {
		return Result{}, fmt.Errorf("exa fetch: MCP caller unavailable")
	}
	pageURL, err := validateFetchURL(req.URL)
	if err != nil {
		return Result{}, fmt.Errorf("exa: %w", err)
	}
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "web fetch: " + pageURL
	}
	maxCharacters := extraInt(req.Extras, "maxCharacters", "max_characters")
	if maxCharacters <= 0 {
		maxCharacters = defaultExaCharacters
	}
	callCtx, cancel := context.WithTimeout(ctx, fetchTimeout(req))
	defer cancel()
	raw, err := f.caller.CallInternalTool(callCtx, f.server, f.fetchTool, intent, map[string]any{
		"urls":          []string{pageURL},
		"maxCharacters": maxCharacters,
	})
	if err != nil {
		return Result{}, fmt.Errorf("exa: %w", err)
	}
	result, err := normalizeMCPFetchResponse(raw, ExaAdapterName, pageURL)
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateFetchURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("only http and https URLs are allowed")
	}
	return u.String(), nil
}
