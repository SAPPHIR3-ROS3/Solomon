package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"

	"github.com/openai/openai-go/v2"
)

func signatureFetchWeb(targetURL, intent string) {}

const (
	fetchWebDefaultTimeoutS  = webfetch.DefaultTimeoutS
	fetchWebMaxTimeoutSecs   = webfetch.MaxTimeoutSecs
	fetchMarkdownDescription = `Fetch a URL and return its content as Markdown. With web_search_engine = internal, use the host-managed Exa/Parallel MCP fetchers with the native CloakBrowser fallback; otherwise use the legacy HTTP fetcher. HTML pages are converted to CommonMark-style Markdown (headings, links, lists, code). Other common text types (plain, JSON, XML) are returned as fenced code blocks. Maximum legacy response body is 5MB. Only http(s) URLs. Optional timeoutSeconds (default 30, max 120).`
)

type fetchWebArgs struct {
	URL            string `json:"url"`
	TimeoutSeconds *int   `json:"timeoutSeconds,omitempty"`
}

func fetchWebOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("fetchWeb", fetchMarkdownDescription, map[string]any{
		"url": map[string]any{"type": "string", "description": "Fully qualified http or https URL to fetch"},
		"timeoutSeconds": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Optional timeout in seconds (default %d, max %d)", fetchWebDefaultTimeoutS, fetchWebMaxTimeoutSecs),
			"minimum":     1,
			"maximum":     fetchWebMaxTimeoutSecs,
		},
	}, []string{"url"})
}

func appendFetchWebDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureFetchWeb)
	if err != nil {
		return err
	}
	b.addBlock("fetchWeb", fetchMarkdownDescription, sig)
	return nil
}

func execFetchWeb(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a fetchWebArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	a.URL = strings.TrimSpace(a.URL)
	if a.URL == "" {
		return nil, fmt.Errorf("fetchWeb: empty url")
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("fetchWeb: invalid url")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return nil, fmt.Errorf("fetchWeb: only http and https URLs are allowed")
	}
	sec := fetchWebDefaultTimeoutS
	if a.TimeoutSeconds != nil && *a.TimeoutSeconds > 0 {
		sec = *a.TimeoutSeconds
		if sec > fetchWebMaxTimeoutSecs {
			sec = fetchWebMaxTimeoutSecs
		}
	}
	var cfg *config.Root
	if env != nil {
		cfg = env.Cfg
	}
	intent, _ := tooling.ToolIntent(raw)
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()
	var res webfetch.Result
	internalEngine := cfg == nil || strings.EqualFold(cfg.EffectiveWebSearchEngine(), search.InternalEngineName)
	if env != nil && env.WebFetch != nil && internalEngine {
		res, err = env.WebFetch.Fetch(reqCtx, webfetch.Request{
			URL:            a.URL,
			TimeoutSeconds: sec,
			Intent:         intent,
		})
	} else {
		if cfg != nil && strings.EqualFold(cfg.EffectiveWebSearchEngine(), search.InternalEngineName) {
			return nil, fmt.Errorf("fetchWeb: internal web-fetch router unavailable")
		}
		res, err = webfetch.FetchURL(reqCtx, a.URL, sec, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("fetchWeb: %w", err)
	}
	out := map[string]any{
		"url":         res.URL,
		"status":      res.Status,
		"contentType": res.ContentType,
		"markdown":    res.Markdown,
	}
	if res.Title != "" {
		out["title"] = res.Title
	}
	if res.Metadata != nil {
		out["metadata"] = res.Metadata
	}
	return out, nil
}
