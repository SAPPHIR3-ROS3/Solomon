package cursor

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

type DiscardBootstrap struct{}

func (DiscardBootstrap) Print(string) {}

var sidecarAsync struct {
	mu   sync.Mutex
	once sync.Once
	done chan struct{}
	err  error
}

func sidecarConfigured(cfg *config.Root) (*config.Provider, string, bool) {
	if cfg == nil {
		return nil, "", false
	}
	p := config.ProviderByName(cfg, config.ProviderNameCursorAPI)
	if p == nil || !p.IsCursorAPI() || !config.ProviderCredentialsReady(p) {
		return nil, "", false
	}
	return p, strings.TrimSpace(p.APIKey), true
}

func KickSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, out BootstrapIO) {
	if _, _, ok := sidecarConfigured(cfg); !ok {
		return
	}
	if out == nil {
		out = DiscardBootstrap{}
	}
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	mgr := DefaultManager()
	if mgr.ProxyStatus(ctx).Healthy {
		return
	}
	_, apiKey, _ := sidecarConfigured(cfg)
	sidecarAsync.once.Do(func() {
		sidecarAsync.done = make(chan struct{})
		go func() {
			defer close(sidecarAsync.done)
			_, sidecarAsync.err = mgr.Ensure(ctx, apiKey, cwd, out)
		}()
	})
}

func WaitSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, out BootstrapIO) error {
	_, apiKey, ok := sidecarConfigured(cfg)
	if !ok {
		return nil
	}
	if out == nil {
		out = DiscardBootstrap{}
	}
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	mgr := DefaultManager()
	if mgr.ProxyStatus(ctx).Healthy {
		return nil
	}
	KickSidecarIfConfigured(ctx, cfg, cwd, out)
	sidecarAsync.mu.Lock()
	ch := sidecarAsync.done
	sidecarAsync.mu.Unlock()
	if ch == nil {
		_, err := mgr.Ensure(ctx, apiKey, cwd, out)
		return err
	}
	select {
	case <-ch:
		return sidecarAsync.err
	case <-ctx.Done():
		return ctx.Err()
	}
}
