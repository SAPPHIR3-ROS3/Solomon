package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
)

func TestWeb(d Deps) error {
	if d.Cfg == nil || d.Out == nil {
		return fmt.Errorf("/testweb unavailable")
	}
	pctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	engine := strings.TrimSpace(d.Cfg.EffectiveWebSearchEngine())
	extras := tools.MergeWebSearchExtras(d.Cfg, engine, nil)
	_, err := search.Run(pctx, engine, search.Request{
		Query:      "test",
		MaxResults: 1,
		Extras:     extras,
	})
	if err == nil {
		PrintSystem(d.Out, "OK")
		return nil
	}

	PrintSystem(d.Out, "NOT OK\nattempting fallback")

	fbCtx, fbCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer fbCancel()
	_, errDDG := search.Run(fbCtx, "duckduckgo", search.Request{
		Query:      "test",
		MaxResults: 1,
		Extras:     nil,
	})
	if errDDG == nil {
		PrintSystem(d.Out, "OK")
		return nil
	}
	PrintSystem(d.Out, "NOT OK")
	logging.Log(logging.ERROR_LOG_LEVEL, "/testweb search failed", logging.LogOptions{Params: map[string]any{
		"engine": engine, "err": err.Error(), "fallback_engine": "duckduckgo", "fallback_err": errDDG.Error(),
	}})
	return fmt.Errorf("web search test failed: %v; fallback: %w", err, errDDG)
}
