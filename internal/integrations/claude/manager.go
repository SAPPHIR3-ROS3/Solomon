package claude

import (
	"bufio"
	"bytes"
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
	"runtime"
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
	stdin io.WriteCloser
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
	dir, err := sourceDir(ctx)
	if err != nil {
		return "", "", err
	}
	node, err := nodeExecutable()
	if err != nil {
		return "", "", err
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
	cmd := exec.Command(node, "--experimental-strip-types", filepath.Join(dir, "src", "index.ts"))
	cmd.Dir = dir
	cmd.Env = append(withNodePath(cliEnv(), node),
		"SOLOMON_CLAUDE_TOKEN="+token,
		"SOLOMON_CLAUDE_PORT=0",
		"SOLOMON_CLAUDE_CWD="+cwd,
		"SOLOMON_CLAUDE_EXECUTABLE="+cli,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	// The sidecar exits when this pipe closes, so it never outlives Solomon, even after a crash.
	stdin, err := cmd.StdinPipe()
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
	running = &sidecar{cmd: cmd, stdin: stdin, base: base, token: token, cwd: cwd}
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

// Models lists the model IDs the official SDK offers to the logged-in subscription.
func Models(ctx context.Context, cwd string) ([]string, error) {
	base, token, err := Ensure(ctx, cwd)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("Claude sidecar models: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude sidecar models: %s", strings.TrimSpace(body.Error.Message))
	}
	ids := make([]string, 0, len(body.Data))
	for _, model := range body.Data {
		ids = append(ids, model.ID)
	}
	return ids, nil
}

// AccessToken reads the CLI's current subscription token without refreshing it;
// refresh tokens rotate, so only the CLI may spend them. On macOS the CLI usually keeps
// credentials in the login keychain rather than in the credentials file.
func AccessToken(ctx context.Context) (string, error) {
	dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".claude")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".credentials.json"))
	if err != nil && keychainCredentials != nil {
		data, err = keychainCredentials(ctx)
	}
	if err != nil {
		return "", fmt.Errorf("Claude CLI credentials are not readable: %w", err)
	}
	var creds struct {
		OAuth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil || creds.OAuth.AccessToken == "" {
		return "", fmt.Errorf("Claude CLI credentials hold no subscription token")
	}
	if creds.OAuth.ExpiresAt > 0 && time.Now().After(time.UnixMilli(creds.OAuth.ExpiresAt)) {
		return "", fmt.Errorf("Claude CLI token expired; it refreshes on the next Claude Sub chat")
	}
	return creds.OAuth.AccessToken, nil
}

// keychainCredentials reads the item the Claude CLI stores in the macOS login keychain;
// it is nil elsewhere, and tests replace it so they never touch a real login.
var keychainCredentials func(ctx context.Context) ([]byte, error)

func init() {
	if runtime.GOOS == "darwin" {
		keychainCredentials = readKeychain
	}
}

// SetKeychainForTest replaces the keychain reader and returns a function that restores it.
func SetKeychainForTest(read func(ctx context.Context) ([]byte, error)) (restore func()) {
	previous := keychainCredentials
	keychainCredentials = read
	return func() { keychainCredentials = previous }
}

func readKeychain(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return nil, fmt.Errorf("keychain: %w", err)
	}
	return bytes.TrimSpace(out), nil
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

// sourceDir prefers an explicit or repository checkout, so development runs the
// live sources, and otherwise uses the copy embedded in this binary.
func sourceDir(ctx context.Context) (string, error) {
	if dir := strings.TrimSpace(os.Getenv("SOLOMON_CLAUDE_SIDECAR_DIR")); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "src", "index.ts")); err != nil {
			return "", fmt.Errorf("Claude sidecar sources missing in %s", dir)
		}
		return dir, ensureModules(ctx, dir)
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
			// go.mod marks a checkout, so the extracted runtime cache under SOLOMON_HOME never
			// matches here and upgrades always resync the embedded sources.
			_, errMod := os.Stat(filepath.Join(dir, "go.mod"))
			if _, err := os.Stat(filepath.Join(candidate, "src", "index.ts")); err == nil && errMod == nil {
				return candidate, ensureModules(ctx, candidate)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return runtimeDir(ctx)
}

func ensureModules(ctx context.Context, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk")); err == nil {
		return nil
	}
	return npmCI(ctx, dir)
}

// nodeExecutable also checks common install locations, since desktop apps
// launched from a GUI session (notably on macOS) miss the shell's PATH.
func nodeExecutable() (string, error) {
	if path := strings.TrimSpace(os.Getenv("SOLOMON_NODE")); path != "" {
		return path, nil
	}
	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}
	home, _ := os.UserHomeDir()
	for _, path := range []string{"/opt/homebrew/bin/node", "/usr/local/bin/node", filepath.Join(home, ".volta", "bin", "node"), filepath.Join(home, ".local", "bin", "node")} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("Node.js 22.18 or newer is required for Claude Sub: node not found in PATH")
}

// withNodePath puts node's directory first in PATH so npm and its scripts find it.
func withNodePath(env []string, node string) []string {
	dir := filepath.Dir(node)
	for i, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && strings.EqualFold(key, "PATH") {
			env[i] = key + "=" + dir + string(os.PathListSeparator) + value
			return env
		}
	}
	return append(env, "PATH="+dir)
}

func npmCI(ctx context.Context, dir string) error {
	node, err := nodeExecutable()
	if err != nil {
		return err
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		npm = filepath.Join(filepath.Dir(node), "npm")
		if runtime.GOOS == "windows" {
			npm += ".cmd"
		}
	}
	cmd := exec.CommandContext(ctx, npm, "ci", "--no-audit", "--no-fund")
	cmd.Dir = dir
	cmd.Env = withNodePath(os.Environ(), node)
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcess(cmd) }
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
