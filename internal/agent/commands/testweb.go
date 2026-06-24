package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
)

const (
	startupConnectivityTimeout = 12 * time.Second
	connectivityProbeTimeout   = 4 * time.Second
	connectivityProbeRetries   = 2
)

var defaultInternetProbeURLs = []string{
	"https://connectivitycheck.gstatic.com/generate_204",
	"https://www.msftconnecttest.com/connecttest.txt",
	"https://cloudflare.com/cdn-cgi/trace",
}

var internetProbeURLsMu sync.Mutex
var internetProbeURLs = append([]string(nil), defaultInternetProbeURLs...)

func internetProbeURLList() []string {
	internetProbeURLsMu.Lock()
	defer internetProbeURLsMu.Unlock()
	return append([]string(nil), internetProbeURLs...)
}

var replStartupNotice struct {
	mu        sync.Mutex
	pending   string
	interrupt chan struct{}
}

var startupConnectivity struct {
	mu      sync.Mutex
	offline bool
	checked bool
}

func init() {
	replStartupNotice.interrupt = make(chan struct{}, 1)
}

func ReplStartupInterrupt() <-chan struct{} {
	return replStartupNotice.interrupt
}

func notifyReplStartupNotice(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	replStartupNotice.mu.Lock()
	replStartupNotice.pending = msg
	replStartupNotice.mu.Unlock()
	select {
	case replStartupNotice.interrupt <- struct{}{}:
	default:
	}
}

func TakeReplStartupNotice(out io.Writer) bool {
	replStartupNotice.mu.Lock()
	pending := replStartupNotice.pending
	replStartupNotice.pending = ""
	replStartupNotice.mu.Unlock()
	if pending == "" {
		return false
	}
	PrintSystem(out, pending)
	return true
}

func DrainReplStartupInterrupt() {
	select {
	case <-replStartupNotice.interrupt:
	default:
	}
}

func PrepareReplStartupNotice(out io.Writer) {
	if TakeReplStartupNotice(out) {
		DrainReplStartupInterrupt()
	}
}

func BeginStartupConnectivityCheck(ctx context.Context, cfg *config.Root) {
	if cfg == nil || config.NeedsOnboard(cfg) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		offline := !InternetReachable(ctx, cfg)
		setStartupOffline(offline)
		if !offline {
			return
		}
		if msg := FormatOfflineNotice(cfg); msg != "" {
			notifyReplStartupNotice(msg)
		}
	}()
}

func setStartupOffline(offline bool) {
	startupConnectivity.mu.Lock()
	startupConnectivity.offline = offline
	startupConnectivity.checked = true
	startupConnectivity.mu.Unlock()
}

func StartupWasOffline() bool {
	startupConnectivity.mu.Lock()
	defer startupConnectivity.mu.Unlock()
	return startupConnectivity.checked && startupConnectivity.offline
}

func clearStartupOffline() {
	startupConnectivity.mu.Lock()
	startupConnectivity.offline = false
	startupConnectivity.mu.Unlock()
}

func SetStartupOfflineForTest(offline bool) {
	setStartupOffline(offline)
}

func ClearStartupOfflineForTest() {
	clearStartupOffline()
}

func InternetReachable(ctx context.Context, cfg *config.Root) bool {
	_ = cfg
	pctx, cancel := context.WithTimeout(ctx, startupConnectivityTimeout)
	defer cancel()
	for attempt := 0; attempt < connectivityProbeRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-pctx.Done():
				return false
			case <-time.After(400 * time.Millisecond):
			}
		}
		for _, rawURL := range internetProbeURLList() {
			if probeURLReachable(pctx, rawURL) {
				return true
			}
		}
	}
	return false
}

func probeURLReachable(ctx context.Context, rawURL string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, connectivityProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	code := resp.StatusCode
	return code >= 200 && code < 400
}

func SetInternetProbeURLsForTest(urls []string) {
	internetProbeURLsMu.Lock()
	defer internetProbeURLsMu.Unlock()
	if len(urls) == 0 {
		internetProbeURLs = append([]string(nil), defaultInternetProbeURLs...)
		return
	}
	internetProbeURLs = append([]string(nil), urls...)
}

func ResetInternetProbeURLsForTest() {
	internetProbeURLsMu.Lock()
	defer internetProbeURLsMu.Unlock()
	internetProbeURLs = append([]string(nil), defaultInternetProbeURLs...)
}

func RemoteMCPServerNames() []string {
	cfg, err := mcp.LoadConfig()
	if err != nil || cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		if s.NeedsInternet() {
			names = append(names, s.Name)
		}
	}
	return names
}

func FormatOfflineNotice(cfg *config.Root) string {
	if cfg == nil {
		return ""
	}
	var items []string
	if cfg.WebSearchNeedsInternet() {
		items = append(items, "- web search")
	}
	if remoteMCP := RemoteMCPServerNames(); len(remoteMCP) > 0 {
		items = append(items, "- remote MCP servers: "+strings.Join(remoteMCP, ", "))
	}
	if remoteProviders := config.RemoteProviderNames(cfg); len(remoteProviders) > 0 {
		items = append(items, "- remote providers: "+strings.Join(remoteProviders, ", "))
	}
	if len(items) == 0 {
		return "No internet connection detected."
	}
	var b strings.Builder
	b.WriteString("No internet connection detected.")
	b.WriteString("\n\nUntil connectivity is restored, the following will be unavailable:")
	for _, item := range items {
		b.WriteByte('\n')
		b.WriteString(item)
	}
	return b.String()
}

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
