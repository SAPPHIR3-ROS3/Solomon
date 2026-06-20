package providersetup

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func RunProviderSetupByKind(ctx context.Context, pio config.PromptIO, cfg *config.Root, existing *config.Root, kind int, opts config.ProviderSetupOpts) (*config.ProviderSetupResult, error) {
	if kind == config.ProviderKindCursorAPI {
		return setupCursorAPI(ctx, pio, cfg, existing, opts)
	}
	return config.RunProviderSetupByKind(ctx, pio, cfg, existing, kind, opts)
}

type bootstrapOut struct {
	out io.Writer
}

func (c bootstrapOut) Print(msg string) {
	if c.out == nil {
		c.out = os.Stdout
	}
	fmt.Fprintln(c.out, msg)
}

func setupCursorAPI(ctx context.Context, pio config.PromptIO, cfg *config.Root, existing *config.Root, opts config.ProviderSetupOpts) (*config.ProviderSetupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	k, err := readCursorAPIKey(pio, opts)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	mgr := cursorint.DefaultManager()
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	rawURL, err := mgr.Ensure(ctx, k, cwd, false, bootstrapOut{out: out})
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "cursor API provider setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	baseURL, err := config.NormalizeAPIBase(rawURL)
	if err != nil {
		return nil, err
	}
	prov := config.Provider{
		Name:        config.ProviderNameCursorAPI,
		BaseURL:     baseURL,
		APIKey:      k,
		AuthKind:    config.AuthKindCursorAPI,
		APIProtocol: config.APIProtocolOpenAI,
	}
	ids, err := modelsapi.List(prov.BaseURL, k)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "cursor API connection check failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, fmt.Errorf("connection check failed: %w", err)
	}
	ids = cursorint.FilterModelIDs(ids)
	return config.FinalizeProviderSetup(pio, cfg, existing, opts, prov, ids)
}

func readCursorAPIKey(pio config.PromptIO, opts config.ProviderSetupOpts) (string, error) {
	_ = opts
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	for {
		line, err := config.ReadPromptLine(pio, "Cursor API key: ")
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
		fmt.Fprintln(out, "Required: enter a value.")
	}
}
