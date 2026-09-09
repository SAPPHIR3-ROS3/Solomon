package agentruntime

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	sandboxparent "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
)

func (r *Runtime) InitMCP(ctx context.Context) {
	options := &solomonmcp.ManagerOptions{}
	if r != nil && r.ProjRoot != "" {
		if rootPath, err := filepath.Abs(r.ProjRoot); err == nil {
			options.Roots = []*sdkmcp.Root{{
				URI:  (&url.URL{Scheme: "file", Path: rootPath}).String(),
				Name: "project",
			}}
		}
	}
	mgr, err := solomonmcp.StartLazyWithOptions(os.Stderr, options)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP disabled", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return
	}
	r.MCP = mgr
	r.configureWebSearch(mgr)
	go r.connectMCPBackground(ctx, mgr)
}

func (r *Runtime) configureWebSearch(mgr *solomonmcp.Manager) {
	if r == nil || mgr == nil {
		return
	}
	specs := mgr.InternalAdapters()
	servers := make([]search.MCPAdapterServer, 0, len(specs))
	for _, spec := range specs {
		if strings.EqualFold(spec.Adapter, search.CloakAdapterName) {
			// CloakBrowser is a local native fallback now. Do not start a
			// legacy community MCP process even if an old mcp.json still has
			// its entry.
			continue
		}
		servers = append(servers, search.MCPAdapterServer{
			ServerName: spec.ServerName,
			Adapter:    spec.Adapter,
		})
	}
	balanceStore := search.DefaultBalanceStore()
	var nativeSearchFallback search.Engine
	var nativeFetchFallback webfetch.Fetcher
	nativeCloak, cloakErr := cloak.NewClient(cloak.Options{Stderr: os.Stderr})
	if cloakErr != nil {
		logging.Log(logging.INFO_LOG_LEVEL, "native CloakBrowser fallback unavailable", logging.LogOptions{Params: map[string]any{"err": cloakErr.Error()}})
	} else {
		r.Cloak = nativeCloak
		nativeSearchFallback, cloakErr = search.NewNativeCloakAdapter(nativeCloak)
		if cloakErr != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "native CloakBrowser search adapter setup failed", logging.LogOptions{Params: map[string]any{"err": cloakErr.Error()}})
		}
		nativeFetchFallback, cloakErr = webfetch.NewNativeCloakFetcher(nativeCloak)
		if cloakErr != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "native CloakBrowser fetch adapter setup failed", logging.LogOptions{Params: map[string]any{"err": cloakErr.Error()}})
		}
	}
	router, err := search.NewMCPRouter(mgr, servers, search.RouterOptions{Store: balanceStore, Fallback: nativeSearchFallback})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP web-search router setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	} else {
		r.WebSearch = router
	}

	fetchServers := make([]webfetch.MCPAdapterServer, 0, len(specs))
	for _, spec := range specs {
		if strings.EqualFold(spec.Adapter, search.CloakAdapterName) {
			continue
		}
		fetchServers = append(fetchServers, webfetch.MCPAdapterServer{
			ServerName: spec.ServerName,
			Adapter:    spec.Adapter,
		})
	}
	fetchRouter, fetchErr := webfetch.NewMCPRouter(mgr, fetchServers, webfetch.RouterOptions{Store: balanceStore, Fallback: nativeFetchFallback})
	if fetchErr != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP web-fetch router setup failed", logging.LogOptions{Params: map[string]any{"err": fetchErr.Error()}})
	} else {
		r.WebFetch = fetchRouter
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP web adapters configured", logging.LogOptions{Params: map[string]any{"adapters": len(servers), "search": err == nil, "fetch": fetchErr == nil}})
}

func (r *Runtime) connectMCPBackground(ctx context.Context, mgr *solomonmcp.Manager) {
	configured, err := solomonmcp.ConfiguredServerCount()
	if err != nil || configured == 0 {
		_, _, _ = mgr.Connect(ctx)
		return
	}
	servers, tools, err := mgr.Connect(ctx)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP connect failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return
	}
	if servers == 0 {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP no servers connected", logging.LogOptions{Params: map[string]any{"configured": configured}})
		return
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP ready", logging.LogOptions{Params: map[string]any{"servers": servers, "configured": configured, "tools": tools}})
}

func (r *Runtime) Close() error {
	r.releaseReplSessionFileLock()
	var closeErrors []error
	if r != nil && r.MCP != nil {
		if err := r.MCP.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if r != nil && r.Cloak != nil {
		if err := r.Cloak.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if r != nil && r.sandboxRetained {
		sandboxparent.ReleaseGlobal()
		r.sandboxRetained = false
	}
	if r != nil && r.RL != nil {
		_ = r.RL.Terminal.ExitRawMode()
	}
	multiline.EnsureCookedTTY()
	if r != nil {
		pid := tooloutput.CurrentPID()
		projHex := r.ProjHex
		if err := tooloutput.Shutdown(pid, projHex, r.ToolOut); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "tool output shutdown failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}
	return errors.Join(closeErrors...)
}

func (r *Runtime) toolParams() ([]openai.ChatCompletionToolUnionParam, error) {
	tools, err := agenttools.NativeToolParams(r.Mode)
	if err != nil {
		return nil, err
	}
	if agenttools.NormalizeMode(r.Mode) == "agent" && r.Session != nil && r.Session.PlanningActive {
		tools = append(tools, agenttools.PlanningNativeToolParams()...)
	}
	return tools, nil
}
