package server

import (
	"context"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"sync"
	"time"
)

const (
	chatAPIPath         = "/__solomon/projects"
	maxChatRequestBytes = 32 << 20
	maxChatTitleLength  = 200
	chatTitleTimeout    = 30 * time.Second
)

type chatAPI struct {
	daemonCtx      context.Context
	activeMu       sync.Mutex
	active         map[string]*chatRun
	atMentionIndex *atmention.IndexCache
	browsers       *cloak.Sessions
}

func newChatAPI(daemonCtx context.Context) *chatAPI {
	return &chatAPI{
		daemonCtx:      daemonCtx,
		browsers:       cloak.NewSessions(daemonCtx, nil),
		active:         make(map[string]*chatRun),
		atMentionIndex: atmention.NewIndexCache(),
	}
}

func (a *chatAPI) beginRun(key string) (*chatRun, bool) {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	if _, ok := a.active[key]; ok {
		return nil, false
	}
	run := newChatRun(a.daemonCtx)
	a.active[key] = run
	return run, true
}

func (a *chatAPI) finishRun(key string, run *chatRun) {
	run.finish()
	a.activeMu.Lock()
	if a.active[key] == run {
		delete(a.active, key)
	}
	a.activeMu.Unlock()
}

func (a *chatAPI) runActive(key string) bool {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	_, ok := a.active[key]
	return ok
}

func (a *chatAPI) cancelRun(key string) bool {
	a.activeMu.Lock()
	run, ok := a.active[key]
	a.activeMu.Unlock()
	if !ok {
		return false
	}
	run.cancel()
	return true
}

func (a *chatAPI) getRun(key string) *chatRun {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	return a.active[key]
}

func loadChatRuntimeConfig() (*config.Root, *config.Provider, error) {
	cfg, err := config.LoadOptional()
	if err != nil {
		return nil, nil, err
	}
	if config.NeedsOnboard(cfg) {
		return nil, nil, errors.New("config not set up; use /onboard")
	}
	prov, err := config.ResolveProvider(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, prov, nil
}
