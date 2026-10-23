package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	authclaude "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/claude"
	"github.com/gofrs/flock"
)

func TestClaudeSidecarSourcesResolveFromRepo(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "integrations", "claude", "src", "index.ts")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repo root not found")
		}
		root = parent
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = claude.Ensure(ctx, root)
	if err == nil {
		t.Fatal("expected cancelled ensure to fail before serving")
	}
}

func TestClaudeAccessTokenReadsCLICredentialsWithoutRefresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	// Never reach the real macOS keychain, and never print token values in failures.
	keychain := []byte(nil)
	defer claude.SetKeychainForTest(func(context.Context) ([]byte, error) {
		if keychain == nil {
			return nil, errors.New("no keychain item")
		}
		return keychain, nil
	})()
	if got, err := claude.AccessToken(t.Context()); got != "" || err == nil {
		t.Fatalf("missing credentials returned a token=%t err=%v", got != "", err)
	}
	keychain = []byte(`{"claudeAiOauth":{"accessToken":"keychain-access"}}`)
	if got, err := claude.AccessToken(t.Context()); got != "keychain-access" || err != nil {
		t.Fatalf("keychain fallback matched=%t err=%v", got == "keychain-access", err)
	}
	write := func(expires time.Time) {
		body := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"cli-access","refreshToken":"cli-refresh","expiresAt":%d}}`, expires.UnixMilli())
		if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(time.Now().Add(time.Hour))
	if got, err := claude.AccessToken(t.Context()); got != "cli-access" || err != nil {
		t.Fatalf("credentials file matched=%t err=%v", got == "cli-access", err)
	}
	write(time.Now().Add(-time.Hour))
	if got, err := claude.AccessToken(t.Context()); got != "" || err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired credentials returned a token=%t err=%v", got != "", err)
	}
}

func TestClaudeCLIStatusAndLoginUseSubscription(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a shell script")
	}
	bin := t.TempDir()
	state := filepath.Join(bin, "logged-in")
	script := "#!/bin/sh\n" +
		"[ -n \"$ANTHROPIC_API_KEY\" ] && { echo leaked >&2; exit 3; }\n" +
		"[ -n \"$anthropic_api_key\" ] && { echo leaked >&2; exit 3; }\n" +
		"case \"$1 $2\" in\n" +
		"'auth status') if [ -f " + state + " ]; then echo '{\"loggedIn\":true,\"authMethod\":\"claude.ai\",\"email\":\"a@b.c\"}'; else echo '{\"loggedIn\":false,\"authMethod\":\"none\"}'; exit 1; fi ;;\n" +
		"'auth login') [ \"$3\" = --claudeai ] && : > " + state + " ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "must-not-reach-cli")
	// Windows reads environment names case-insensitively, so other spellings must not leak either.
	t.Setenv("anthropic_api_key", "must-not-reach-cli")
	status, err := claude.Status(t.Context())
	if err != nil || status.Subscription() {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := claude.Login(t.Context(), strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
	status, err = claude.Status(t.Context())
	if err != nil || !status.Subscription() || status.Email != "a@b.c" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestClaudeSubIsReadyWithoutSolomonTokens(t *testing.T) {
	p, err := config.NewClaudeSubProvider(authclaude.TokenSet{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsClaudeSub() || !config.ProviderCredentialsReady(&p) {
		t.Fatalf("provider=%+v", p)
	}
	got := config.FilterClaudeSubModels([]string{"default", "opus", "claude-opus-5", "claude-opus-5", " claude-haiku-4-5 "})
	if strings.Join(got, ",") != "claude-opus-5,claude-haiku-4-5" {
		t.Fatalf("models=%v", got)
	}
}

func TestClaudeRuntimeRetriesAFailedInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake npm is a shell script")
	}
	bin := t.TempDir()
	runs := filepath.Join(bin, "runs")
	// The fake npm leaves the SDK directory behind and fails on its first run, as an interrupted npm ci can.
	script := "#!/bin/sh\n" +
		"echo run >> " + runs + "\n" +
		"mkdir -p node_modules/@anthropic-ai/claude-agent-sdk\n" +
		"touch node_modules/@anthropic-ai/claude-agent-sdk/package.json\n" +
		"[ $(wc -l < " + runs + ") -gt 1 ] || exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Setenv("SOLOMON_NODE", filepath.Join(bin, "node"))
	t.Setenv("SOLOMON_HOME", t.TempDir())
	if _, err := claude.RuntimeDirForTest(t.Context()); err == nil {
		t.Fatal("first install should fail")
	}
	for range 2 {
		if _, err := claude.RuntimeDirForTest(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(runs)
	if got := strings.Count(string(data), "run"); got != 2 {
		t.Fatalf("npm runs=%d, want a retry after the failure and none once installed", got)
	}
	// A valid stamp must not hide dependencies removed after the install.
	dir, err := claude.RuntimeDirForTest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if _, err := claude.RuntimeDirForTest(t.Context()); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(runs)
	if got := strings.Count(string(data), "run"); got != 3 {
		t.Fatalf("npm runs=%d, want a reinstall after node_modules was removed", got)
	}
}

func TestClaudeRuntimeLockHonorsCancellation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	dir := filepath.Join(home, "integrations", "claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	held := flock.New(filepath.Join(dir, ".install.lock"))
	if err := held.Lock(); err != nil {
		t.Fatal(err)
	}
	defer held.Unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := claude.RuntimeDirForTest(ctx); err == nil {
		t.Fatal("expected a context error while another process holds the lock")
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("lock wait ignored cancellation for %s", waited)
	}
}
