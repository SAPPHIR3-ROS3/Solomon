package providersetup

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func RunProviderSetupByKind(ctx context.Context, pio config.PromptIO, cfg *config.Root, existing *config.Root, kind int, opts config.ProviderSetupOpts) (*config.ProviderSetupResult, error) {
	if kind == config.ProviderKindCursorAPI {
		return setupCursorAPI(ctx, pio, cfg, existing, opts)
	}
	if kind == config.ProviderKindCursorSub {
		return setupCursorSub(ctx, pio, cfg, existing, opts)
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
	prov := config.Provider{
		Name:        config.ProviderNameCursorAPI,
		APIKey:      k,
		AuthKind:    config.AuthKindCursorAPI,
		APIProtocol: config.APIProtocolOpenAI,
	}
	return finishCursorProvider(ctx, pio, cfg, existing, opts, prov, k)
}

func setupCursorSub(ctx context.Context, pio config.PromptIO, cfg *config.Root, existing *config.Root, opts config.ProviderSetupOpts) (*config.ProviderSetupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	tokens, err := cursorauth.Login(ctx, out)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "Cursor Sub login failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	key, err := cursorauth.MintUserAPIKey(ctx, tokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("Cursor Sub Agent API key: %w", err)
	}
	tokens.APIKey = key
	prov := config.Provider{
		Name:        config.ProviderNameCursorSub,
		APIProtocol: config.APIProtocolOpenAI,
	}
	config.ApplyCursorOAuthTokens(&prov, tokens)
	prov.BaseURL = config.CursorSubChatBase()
	ids := cursorauth.DefaultModelIDs()
	return config.FinalizeProviderSetup(pio, cfg, existing, opts, prov, ids)
}

func finishCursorProvider(ctx context.Context, pio config.PromptIO, cfg *config.Root, existing *config.Root, opts config.ProviderSetupOpts, prov config.Provider, bearer string) (*config.ProviderSetupResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	out := pio.Out
	if out == nil {
		out = os.Stdout
	}
	rawURL, err := cursorint.DefaultManager().Ensure(ctx, bearer, cwd, false, bootstrapOut{out: out})
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "cursor API provider setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	baseURL, err := config.NormalizeAPIBase(rawURL)
	if err != nil {
		return nil, err
	}
	prov.BaseURL = baseURL
	ids, err := modelsapi.List(prov.BaseURL, bearer)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "cursor API connection check failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		ids = cursorint.DefaultModelIDs()
	} else {
		ids = cursorint.FilterModelIDs(ids)
	}
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
