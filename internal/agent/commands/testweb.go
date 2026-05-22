package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/search"
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
	return nil
}
