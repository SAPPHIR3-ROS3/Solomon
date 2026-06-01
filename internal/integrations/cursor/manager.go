package cursor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var (
	mu      sync.Mutex
	running *processState
)

type processState struct {
	cmd                      *exec.Cmd
	port                     int
	dir                      string
	apiKey                   string
	cwd                      string
	allowCursorInternalTools bool
}

type Manager struct {
	Port int
}

func DefaultManager() *Manager {
	return &Manager{Port: DefaultPort}
}

func (m *Manager) Ensure(ctx context.Context, apiKey, cwd string, allowCursorInternalTools bool, out BootstrapIO) (baseURL string, err error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", fmt.Errorf("missing Cursor API key")
	}
	dir, err := ResolveInstallDir()
	if err != nil {
		return "", err
	}
	if !InstallDirReady(dir) {
		if err := Bootstrap(out, dir); err != nil {
			return "", err
		}
	} else if _, err := os.Stat(filepath.Join(NodeModulesDir(dir), "@cursor", "sdk")); err != nil {
		if err := Bootstrap(out, dir); err != nil {
			return "", err
		}
	}
	port := m.Port
	if port <= 0 {
		port = DefaultPort
	}
	cwd = sidecarCWD(cwd)
	mu.Lock()
	defer mu.Unlock()
	if running != nil && running.apiKey == apiKey && running.dir == dir && running.port == port && running.allowCursorInternalTools == allowCursorInternalTools {
		if healthOK(ctx, port) {
			return DefaultBaseURL(port), nil
		}
		if processAlive(running) {
			if waitHealth(ctx, port, 15*time.Second) {
				return DefaultBaseURL(port), nil
			}
			return DefaultBaseURL(port), nil
		}
		running = nil
	}
	if running != nil {
		stopLocked()
	}
	if healthOK(ctx, port) {
		if running == nil {
			running = &processState{port: port, dir: dir, apiKey: apiKey, cwd: cwd, allowCursorInternalTools: allowCursorInternalTools}
		}
		return DefaultBaseURL(port), nil
	}
	if err := startLocked(dir, apiKey, cwd, allowCursorInternalTools, port); err != nil {
		return "", err
	}
	if waitHealth(ctx, port, 45*time.Second) {
		return DefaultBaseURL(port), nil
	}
	stopLocked()
	return "", fmt.Errorf("cursor API proxy failed health check on port %d", port)
}

func (m *Manager) Stop() {
	mu.Lock()
	defer mu.Unlock()
	stopLocked()
}

type ProxyStatus struct {
	Port       int
	BaseURL    string
	InstallDir string
	Managed    bool
	Healthy    bool
}

func (m *Manager) ProxyStatus(ctx context.Context) ProxyStatus {
	port := m.Port
	if port <= 0 {
		port = DefaultPort
	}
	dir, _ := ResolveInstallDir()
	st := ProxyStatus{
		Port:       port,
		BaseURL:    DefaultBaseURL(port),
		InstallDir: dir,
		Healthy:    healthOK(ctx, port),
	}
	mu.Lock()
	st.Managed = running != nil && running.port == port && processAlive(running)
	mu.Unlock()
	return st
}

func nodeExecutable() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SOLOMON_NODE")); p != "" {
		return p, nil
	}
	return exec.LookPath("node")
}

func sidecarLogFile() (*os.File, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(logDir, "cursor-sidecar.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func startLocked(dir, apiKey, cwd string, allowCursorInternalTools bool, port int) error {
	stopLocked()
	node, err := nodeExecutable()
	if err != nil {
		return err
	}
	entry := EntryScript(dir)
	cmd := exec.Command(node, entry)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"CURSOR_API_KEY="+apiKey,
		fmt.Sprintf("CURSOR_API_PORT=%d", port),
		"CURSOR_API_CWD="+cwd,
		fmt.Sprintf("CURSOR_API_ALLOW_INTERNAL_TOOLS=%t", allowCursorInternalTools),
	)
	if logFile, err := sidecarLogFile(); err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start cursor proxy: %w", err)
	}
	running = &processState{cmd: cmd, port: port, dir: dir, apiKey: apiKey, cwd: cwd, allowCursorInternalTools: allowCursorInternalTools}
	go watchSidecarProcess(cmd, dir, apiKey, cwd, allowCursorInternalTools, port)
	return nil
}

func watchSidecarProcess(cmd *exec.Cmd, dir, apiKey, cwd string, allowCursorInternalTools bool, port int) {
	_ = cmd.Wait()
	mu.Lock()
	shouldRestart := running != nil && running.cmd == cmd
	if shouldRestart {
		running = nil
	}
	mu.Unlock()
	if !shouldRestart {
		return
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "cursor API sidecar exited; restarting", logging.LogOptions{Params: map[string]any{"port": port}})
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if running != nil {
		return
	}
	if err := startLocked(dir, apiKey, cwd, allowCursorInternalTools, port); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "cursor API sidecar restart failed", logging.LogOptions{Params: map[string]any{"err": err.Error(), "port": port}})
	}
}

func stopLocked() {
	if running == nil || running.cmd == nil || running.cmd.Process == nil {
		running = nil
		return
	}
	_ = running.cmd.Process.Kill()
	_ = running.cmd.Wait()
	running = nil
}

func processAlive(ps *processState) bool {
	if ps == nil || ps.cmd == nil || ps.cmd.Process == nil {
		return false
	}
	return ps.cmd.ProcessState == nil
}

func waitHealth(ctx context.Context, port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if healthOK(ctx, port) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func healthOK(ctx context.Context, port int) bool {
	u := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}
