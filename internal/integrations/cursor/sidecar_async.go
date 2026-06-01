package cursor

import (
	"context"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type DiscardBootstrap struct{}

func (DiscardBootstrap) Print(string) {}

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

func sidecarCWD(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	return cwd
}

func KickSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, out BootstrapIO) {
	_, apiKey, ok := sidecarConfigured(cfg)
	if !ok {
		return
	}
	allowCursorInternalTools := cfg.Tools.CursorInternalTools
	if out == nil {
		out = DiscardBootstrap{}
	}
	mgr := DefaultManager()
	cwd = sidecarCWD(cwd)
	go func() {
		_, _ = mgr.Ensure(ctx, apiKey, cwd, allowCursorInternalTools, out)
	}()
}

func WaitSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, out BootstrapIO) error {
	_, apiKey, ok := sidecarConfigured(cfg)
	if !ok {
		return nil
	}
	if out == nil {
		out = DiscardBootstrap{}
	}
	allowCursorInternalTools := cfg.Tools.CursorInternalTools
	mgr := DefaultManager()
	_, err := mgr.Ensure(ctx, apiKey, sidecarCWD(cwd), allowCursorInternalTools, out)
	return err
}

func isSidecarNetFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, sub := range []string{
		"connection refused",
		"connection reset",
		"forcibly closed",
		"broken pipe",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func ReviveSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, err error) {
	if !isSidecarNetFailure(err) {
		return
	}
	_, apiKey, ok := sidecarConfigured(cfg)
	if !ok {
		return
	}
	_, _ = DefaultManager().Ensure(ctx, apiKey, sidecarCWD(cwd), cfg.Tools.CursorInternalTools, DiscardBootstrap{})
}
