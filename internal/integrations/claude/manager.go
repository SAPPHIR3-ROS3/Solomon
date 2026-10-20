package claude

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	running *sidecar
)

type sidecar struct {
	cmd   *exec.Cmd
	base  string
	token string
	cwd   string
}

type readyLine struct {
	Port    int    `json:"port"`
	BaseURL string `json:"baseURL"`
}

func Ensure(ctx context.Context, cwd string) (baseURL, token string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	mu.Lock()
	defer mu.Unlock()
	if running != nil && running.cwd == cwd && health(ctx, running.base, running.token) == nil {
		return running.base, running.token, nil
	}
	stopLocked()
	dir, err := sourceDir()
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk")); err != nil {
		if err := npmCI(ctx, dir); err != nil {
			return "", "", err
		}
	}
	cli := installedCLI()
	if cli == "" {
		cli, err = bundledCLI(dir)
		if err != nil {
			return "", "", err
		}
	}
	token, err = newToken()
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command("node", "--experimental-strip-types", filepath.Join(dir, "src", "index.ts"))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"SOLOMON_CLAUDE_TOKEN="+token,
		"SOLOMON_CLAUDE_PORT=0",
		"SOLOMON_CLAUDE_CWD="+cwd,
		"SOLOMON_CLAUDE_EXECUTABLE="+cli,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	logFile, _ := sidecarLog()
	if logFile != nil {
		cmd.Stderr = logFile
	} else {
		cmd.Stderr = io.Discard
	}
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("start Claude sidecar: %w", err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	base, err := readReady(readyCtx, stdout)
	if err != nil {
		_ = killProcess(cmd)
		return "", "", err
	}
	if err := health(ctx, base, token); err != nil {
		_ = killProcess(cmd)
		return "", "", err
	}
	running = &sidecar{cmd: cmd, base: base, token: token, cwd: cwd}
	go func(cmd *exec.Cmd) {
		_ = cmd.Wait()
		mu.Lock()
		if running != nil && running.cmd == cmd {
			running = nil
		}
		mu.Unlock()
	}(cmd)
	return base, token, nil
}

func stopLocked() {
	if running == nil {
		return
	}
	_ = killProcess(running.cmd)
	running = nil
}

func readReady(ctx context.Context, r io.Reader) (string, error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(r).ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		var ready readyLine
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &ready); err != nil || ready.BaseURL == "" {
			errCh <- fmt.Errorf("Claude sidecar ready line: %s", strings.TrimSpace(line))
			return
		}
		ch <- ready.BaseURL
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("Claude sidecar did not become ready: %w", ctx.Err())
	case err := <-errCh:
		return "", fmt.Errorf("Claude sidecar did not become ready: %w", err)
	case base := <-ch:
		return base, nil
	}
}

func health(ctx context.Context, base, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/v1")+"/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Claude sidecar health status %d", resp.StatusCode)
	}
	return nil
}

func sourceDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("SOLOMON_CLAUDE_SIDECAR_DIR")); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "src", "index.ts")); err != nil {
			return "", fmt.Errorf("Claude sidecar sources missing in %s", dir)
		}
		return dir, nil
	}
	starts := []string{}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		dir := start
		for i := 0; i < 8; i++ {
			candidate := filepath.Join(dir, "integrations", "claude")
			if _, err := os.Stat(filepath.Join(candidate, "src", "index.ts")); err == nil {
				return candidate, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("Claude sidecar sources not found")
}

func npmCI(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "npm", "ci", "--no-audit", "--no-fund")
	cmd.Dir = dir
	configureProcess(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Claude sidecar: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func newToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func sidecarLog() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".solomon", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "claude-sidecar.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
