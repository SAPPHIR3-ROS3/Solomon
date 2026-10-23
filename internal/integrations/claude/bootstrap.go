package claude

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	sidecarsrc "github.com/SAPPHIR3-ROS3/Solomon/v2026/integrations/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// runtimeDir extracts the embedded sidecar into Solomon home so installed
// clients, such as the desktop app, run it outside a repository checkout.
func runtimeDir(ctx context.Context) (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "integrations", "claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// Another Solomon process may be running npm ci here; waiting must not outlive the caller.
	lock := flock.New(filepath.Join(dir, ".install.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return "", err
	}
	if !locked {
		return "", ctx.Err()
	}
	defer lock.Unlock()
	err = fs.WalkDir(sidecarsrc.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == "." {
			return walkErr
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := sidecarsrc.Files.ReadFile(path)
		if err != nil {
			return err
		}
		if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
			return nil
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		return "", fmt.Errorf("extract Claude sidecar: %w", err)
	}
	lockfile, err := sidecarsrc.Files.ReadFile("package-lock.json")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(lockfile)
	want := hex.EncodeToString(sum[:])
	stamp := filepath.Join(dir, ".installed")
	// The stamp proves npm ci once succeeded; the SDK check catches node_modules removed since.
	_, sdkErr := os.Stat(filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk", "package.json"))
	if current, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(current)) == want && sdkErr == nil {
		return dir, nil
	}
	// The stamp is written only after npm ci succeeds, so a failed or interrupted install is retried
	// instead of leaving a partial node_modules that looks complete.
	_ = os.Remove(stamp)
	if err := npmCI(ctx, dir); err != nil {
		return "", err
	}
	tmp := stamp + ".tmp"
	if err := os.WriteFile(tmp, []byte(want+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, stamp); err != nil {
		return "", err
	}
	return dir, nil
}

// RuntimeDirForTest exposes runtime extraction to the external test package.
func RuntimeDirForTest(ctx context.Context) (string, error) { return runtimeDir(ctx) }

func installedCLI() string {
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	names := []string{filepath.Join(home, ".local", "bin", "claude"), filepath.Join(home, ".claude", "local", "claude"), filepath.Join(home, ".npm-global", "bin", "claude")}
	if runtime.GOOS == "windows" {
		names = append(names, filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"), filepath.Join(home, ".local", "bin", "claude.exe"))
	}
	for _, path := range names {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
			return path
		}
	}
	return ""
}

func bundledCLI(dir string) (string, error) {
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk-"+platform+"-"+arch)
	if runtime.GOOS == "linux" {
		if matches, _ := filepath.Glob("/lib/ld-musl-*.so.1"); len(matches) > 0 {
			base += "-musl"
		}
	}
	path := filepath.Join(base, name)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", fmt.Errorf("official Claude CLI missing from SDK optional package for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return path, nil
}

// resolveCLI prefers an existing Claude CLI over the SDK's bundled one.
func resolveCLI(ctx context.Context) (string, error) {
	if cli := installedCLI(); cli != "" {
		return cli, nil
	}
	dir, err := sourceDir(ctx)
	if err != nil {
		return "", err
	}
	return bundledCLI(dir)
}

var apiCredentialEnv = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN"}

// cliEnv drops API credentials so the CLI always authenticates with the subscription login.
// Names compare case-insensitively because Windows environment variables are.
func cliEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !isAPICredentialEnv(key) {
			env = append(env, entry)
		}
	}
	return env
}

func isAPICredentialEnv(key string) bool {
	for _, name := range apiCredentialEnv {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

type AuthStatus struct {
	LoggedIn   bool   `json:"loggedIn"`
	AuthMethod string `json:"authMethod"`
	Email      string `json:"email"`
}

// Subscription reports whether the login can serve Claude Sub chats.
func (s AuthStatus) Subscription() bool {
	return s.LoggedIn && s.AuthMethod == "claude.ai"
}

func Status(ctx context.Context) (AuthStatus, error) {
	cli, err := resolveCLI(ctx)
	if err != nil {
		return AuthStatus{}, err
	}
	cmd := exec.CommandContext(ctx, cli, "auth", "status", "--json")
	cmd.Env = cliEnv()
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcess(cmd) }
	// A logged-out CLI exits non-zero but still prints its status.
	out, runErr := cmd.Output()
	var status AuthStatus
	if err := json.Unmarshal(out, &status); err != nil {
		if runErr != nil {
			return AuthStatus{}, fmt.Errorf("official Claude CLI status: %w", runErr)
		}
		return AuthStatus{}, fmt.Errorf("official Claude CLI status: %w", err)
	}
	return status, nil
}

func Login(ctx context.Context, in io.Reader, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cli, err := resolveCLI(ctx)
	if err != nil {
		return err
	}
	if out == nil {
		out = io.Discard
	}
	cmd := exec.CommandContext(ctx, cli, "auth", "login", "--claudeai")
	cmd.Env = cliEnv()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcess(cmd) }
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("official Claude CLI login: %w", err)
	}
	status, err := Status(ctx)
	if err != nil {
		return err
	}
	if !status.Subscription() {
		return fmt.Errorf("official Claude CLI login did not produce a Claude subscription session")
	}
	return nil
}
