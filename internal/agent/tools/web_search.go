package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureWebSearch(query, intent string) {}

const (
	webSearchDefaultTimeoutS = 30
	webSearchMaxTimeoutSecs  = 120
	webSearchMaxResultsCap   = 50
	webSearchDescription     = `Runs a web search. Set engine to internal to use the host-managed Exa/Parallel MCP router with the native CloakBrowser fallback; internal MCP servers are configured in mcp.json and require no embedded API key. Legacy direct engines remain available during migration: duckduckgo, searxng, googlepse, brave, bing. maxResults default 10 (cap 50).`
)

type webSearchArgs struct {
	Query          string         `json:"query"`
	Intent         string         `json:"intent"`
	Engine         string         `json:"engine,omitempty"`
	MaxResults     *int           `json:"maxResults,omitempty"`
	Extras         map[string]any `json:"extras,omitempty"`
	TimeoutSeconds *int           `json:"timeoutSeconds,omitempty"`
}

func MergeWebSearchExtras(cfg *config.Root, engineKey string, tool map[string]any) map[string]any {
	ek := strings.ToLower(strings.TrimSpace(engineKey))
	out := map[string]any{}
	if cfg != nil {
		switch ek {
		case "searxng":
			if s := strings.TrimSpace(cfg.WebSearchBaseURL); s != "" {
				out["baseURL"] = s
			}
		case "googlepse":
			if s := strings.TrimSpace(cfg.WebSearchAPIKey); s != "" {
				out["apiKey"] = s
			}
			if s := strings.TrimSpace(cfg.WebSearchCX); s != "" {
				out["cx"] = s
			}
		case "brave", "bing":
			if s := strings.TrimSpace(cfg.WebSearchAPIKey); s != "" {
				out["apiKey"] = s
			}
		}
	}
	for k, v := range tool {
		out[k] = v
	}
	return out
}

func webSearchOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("webSearch", webSearchDescription, map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "Search query text",
		},
		"engine": map[string]any{
			"type":        "string",
			"description": `Overrides effective engine if set (default from config or duckduckgo during migration; use internal for the MCP router).`,
		},
		"maxResults": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Maximum hits to return (default 10, cap %d)", webSearchMaxResultsCap),
			"minimum":     1,
			"maximum":     webSearchMaxResultsCap,
		},
		"extras": map[string]any{
			"type":                 "object",
			"description":          `Optional per-call overrides: provider-specific fields for legacy engines; searchQueries/sessionID/modelName for Parallel; searchURL for the Cloak fallback.`,
			"additionalProperties": true,
		},
		"timeoutSeconds": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Optional HTTP timeout in seconds (default %d, max %d)", webSearchDefaultTimeoutS, webSearchMaxTimeoutSecs),
			"minimum":     1,
			"maximum":     webSearchMaxTimeoutSecs,
		},
	}, []string{"query"})
}

func appendWebSearchDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureWebSearch)
	if err != nil {
		return err
	}
	b.addBlock("webSearch", webSearchDescription, sig)
	return nil
}

func execWebSearch(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a webSearchArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	a.Query = strings.TrimSpace(a.Query)
	a.Engine = strings.TrimSpace(a.Engine)
	if a.Query == "" {
		return nil, fmt.Errorf("webSearch: empty query")
	}
	engine := strings.TrimSpace(a.Engine)
	if engine == "" {
		if env != nil && env.Cfg != nil {
			engine = env.Cfg.EffectiveWebSearchEngine()
		} else {
			engine = config.DefaultWebSearchEngine
		}
	}
	sec := webSearchDefaultTimeoutS
	if a.TimeoutSeconds != nil && *a.TimeoutSeconds > 0 {
		sec = *a.TimeoutSeconds
		if sec > webSearchMaxTimeoutSecs {
			sec = webSearchMaxTimeoutSecs
		}
	}
	max := 10
	if a.MaxResults != nil && *a.MaxResults > 0 {
		max = *a.MaxResults
		if max > webSearchMaxResultsCap {
			max = webSearchMaxResultsCap
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()

	var cfg *config.Root
	if env != nil {
		cfg = env.Cfg
	}
	extras := MergeWebSearchExtras(cfg, engine, a.Extras)

	request := search.Request{
		Query:      a.Query,
		MaxResults: max,
		Intent:     a.Intent,
		Extras:     extras,
	}
	var out search.Response
	var err error
	if strings.EqualFold(engine, search.InternalEngineName) && env != nil && env.WebSearch != nil {
		out, err = search.RunEngine(reqCtx, engine, env.WebSearch, request)
	} else {
		out, err = search.Run(reqCtx, engine, request)
	}
	if err != nil {
		return nil, fmt.Errorf("webSearch: %w", err)
	}
	m := map[string]any{
		"engine":  out.Engine,
		"hits":    out.Hits,
		"hasMore": out.HasMore,
	}
	if out.Metadata != nil {
		m["metadata"] = out.Metadata
	}
	if out.SearxBaseURL != "" {
		m["searxBaseURL"] = out.SearxBaseURL
	}
	return m, nil
}
