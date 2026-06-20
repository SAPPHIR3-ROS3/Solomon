package research

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

func mergeSearchExtras(cfg *config.Root, engineKey string) map[string]any {
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
	return out
}

func runSearch(ctx context.Context, cfg *config.Root, engine, query string, maxResults int) (search.Response, error) {
	if strings.TrimSpace(query) == "" {
		return search.Response{}, fmt.Errorf("empty query")
	}
	if engine == "" {
		engine = cfg.EffectiveWebSearchEngine()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := search.Run(reqCtx, engine, search.Request{
		Query:      query,
		MaxResults: maxResults,
		Extras:     mergeSearchExtras(cfg, engine),
	})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "research web search failed", logging.LogOptions{Params: map[string]any{"engine": engine, "query": query, "err": err.Error()}})
	}
	return resp, err
}

func fetchPage(ctx context.Context, cfg *config.Root, pageURL string) (webfetch.Result, error) {
	res, err := webfetch.FetchURL(ctx, pageURL, webfetch.DefaultTimeoutS, cfg)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "research page fetch failed", logging.LogOptions{Params: map[string]any{"url": pageURL, "err": err.Error()}})
	}
	return res, err
}
