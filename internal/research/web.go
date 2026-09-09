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

func runSearch(ctx context.Context, cfg *config.Root, engine, query string, maxResults int, internalEngine search.Engine) (search.Response, error) {
	if strings.TrimSpace(query) == "" {
		return search.Response{}, fmt.Errorf("empty query")
	}
	if engine == "" {
		engine = cfg.EffectiveWebSearchEngine()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request := search.Request{
		Query:      query,
		MaxResults: maxResults,
		Intent:     "deep research search: " + query,
		Extras:     mergeSearchExtras(cfg, engine),
	}
	var resp search.Response
	var err error
	if internalEngine != nil && strings.EqualFold(engine, search.InternalEngineName) {
		resp, err = search.RunEngine(reqCtx, engine, internalEngine, request)
	} else {
		resp, err = search.Run(reqCtx, engine, request)
	}
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "research web search failed", logging.LogOptions{Params: map[string]any{"engine": engine, "query": query, "err": err.Error()}})
	}
	return resp, err
}

func fetchPage(ctx context.Context, cfg *config.Root, pageURL string, internalFetcher webfetch.Fetcher) (webfetch.Result, error) {
	var res webfetch.Result
	var err error
	if cfg != nil && strings.EqualFold(cfg.EffectiveWebSearchEngine(), search.InternalEngineName) {
		if internalFetcher == nil {
			err = fmt.Errorf("internal web-fetch router unavailable")
		} else {
			res, err = internalFetcher.Fetch(ctx, webfetch.Request{
				URL:            pageURL,
				TimeoutSeconds: webfetch.DefaultTimeoutS,
				Intent:         "deep research page extraction: " + pageURL,
			})
		}
	} else {
		res, err = webfetch.FetchURL(ctx, pageURL, webfetch.DefaultTimeoutS, cfg)
	}
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "research page fetch failed", logging.LogOptions{Params: map[string]any{"url": pageURL, "err": err.Error()}})
	}
	return res, err
}
