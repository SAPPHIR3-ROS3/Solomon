package agentruntime

import (
	"context"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

func (r *Runtime) researchWebResources() (search.Engine, webfetch.Fetcher, func()) {
	if r.MCP == nil || r.Cfg.EffectiveWebSearchEngine() != search.InternalEngineName {
		return r.WebSearch, r.WebFetch, func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := &Runtime{Cfg: r.Cfg, ProjRoot: r.ProjRoot, ProjHex: r.ProjHex}
	r.chatPersistMu.Lock()
	if r.Session != nil {
		worker.Session = &chatstore.Session{ID: r.Session.ID, CreatedAt: r.Session.CreatedAt}
	}
	r.chatPersistMu.Unlock()
	worker.InitMCP(ctx)
	var once sync.Once
	return worker.WebSearch, worker.WebFetch, func() {
		once.Do(func() {
			cancel()
			if worker.MCP != nil {
				_ = worker.MCP.Close()
			}
			if worker.Cloak != nil {
				_ = worker.Cloak.Close()
			}
		})
	}
}
