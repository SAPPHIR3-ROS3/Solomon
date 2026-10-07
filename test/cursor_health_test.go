package test

import (
	"context"
	"encoding/json"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fixtureRuntime(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dist/prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules/@cursor/sdk"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dist/index.js", "dist/prompts/harness.txt", "package.json", "package-lock.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestEnsureVerifiedListener(t *testing.T) {
	for _, mode := range []string{"compatible", "adopted-reuse", "legacy", "bundle", "cwd", "internal", "observability", "credentials", "protocol", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			cursorint.DefaultManager().Stop()
			t.Cleanup(func() { cursorint.DefaultManager().Stop() })
			dir := fixtureRuntime(t)
			t.Setenv("SOLOMON_CURSOR_API_ROOT", dir)
			expected, err := cursorint.ExpectedHealthForTest(dir, dir, false)
			if err != nil {
				t.Fatal(err)
			}
			id := expected
			switch mode {
			case "bundle":
				id.Bundle = "stale"
			case "cwd":
				id.CWD = dir + "-other"
			case "internal":
				id.InternalTools = true
			case "observability":
				id.Observability = false
			case "protocol":
				id.Protocol++
			}
			key := "test-secret"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "legacy" {
					_, _ = w.Write([]byte(`{"ok":true}`))
					return
				}
				if mode == "malformed" {
					_, _ = w.Write([]byte("invalid"))
					return
				}
				proofKey := key
				if mode == "credentials" {
					proofKey = "different-secret"
				}
				_ = json.NewEncoder(w).Encode(cursorint.HealthResponseForTest{OK: true, Identity: id, Proof: cursorint.HealthProofForTest(proofKey, r.URL.Query().Get("nonce"), id)})
			}))
			defer server.Close()
			_, portString, _ := net.SplitHostPort(server.Listener.Addr().String())
			port, _ := strconv.Atoi(portString)
			mgr := &cursorint.Manager{Port: port}

			url, err := mgr.Ensure(context.Background(), key, dir, false, cursorint.DiscardBootstrap{})
			compatible := mode == "compatible" || mode == "adopted-reuse"
			if compatible {
				if err != nil || url != cursorint.DefaultBaseURL(port) {
					t.Fatalf("expected verified reuse: %q %v", url, err)
				}
				if _, err = mgr.Ensure(context.Background(), key, dir, false, cursorint.DiscardBootstrap{}); err != nil {
					t.Fatal(err)
				}
				if _, err = mgr.Ensure(context.Background(), "changed-key", dir, false, cursorint.DiscardBootstrap{}); err == nil {
					t.Fatal("adopted listener accepted changed credentials")
				}
			} else {
				if err == nil || url != "" {
					t.Fatalf("incompatible listener reused: %q %v", url, err)
				}
				if strings.Contains(err.Error(), key) {
					t.Fatal("error exposed API key")
				}
			}
			if !cursorint.HealthOKForTest(context.Background(), port) {
				t.Fatal("external listener was terminated")
			}
		})
	}
}

func TestRuntimeDigestTracksAssets(t *testing.T) {
	dir := fixtureRuntime(t)
	first, err := cursorint.RuntimeDigestForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist/prompts/harness.txt"), []byte("new policy"), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := cursorint.RuntimeDigestForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("prompt update did not invalidate bundle")
	}
}

func TestHealthProofVector(t *testing.T) {
	id := cursorint.HealthIdentityForTest{Protocol: 1, Bundle: "bundle", CWD: "/workspace", InternalTools: false, Observability: true}
	if cursorint.HealthProofForTest("key", "nonce", id) != "a9d6a238cc83623d104d9472076cc1602439f28b1ebfa069bd56bbd8f7873b27" {
		t.Fatal("proof differs from Node HMAC vector")
	}
}

func TestManagedNodeHandshakeLifecycle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node is not installed")
	}
	root, err := filepath.Abs("../integrations/cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !cursorint.InstallDirReady(root) || !cursorint.SDKInstalledForTest(root) {
		t.Skip("build sidecar and install its SDK to run lifecycle test")
	}
	cursorint.DefaultManager().Stop()
	t.Cleanup(func() { cursorint.DefaultManager().Stop() })
	t.Setenv("SOLOMON_CURSOR_API_ROOT", root)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	cwd := t.TempDir()
	mgr := &cursorint.Manager{Port: port}
	ctx := context.Background()
	if _, err = mgr.Ensure(ctx, "test-node-key", cwd, false, cursorint.DiscardBootstrap{}); err != nil {
		t.Fatal(err)
	}
	first := cursorint.ManagedPIDForTest()
	if first == 0 {
		t.Fatal("expected managed Node process")
	}
	if _, err = mgr.Ensure(ctx, "test-node-key", cwd, false, cursorint.DiscardBootstrap{}); err != nil {
		t.Fatal(err)
	}
	if cursorint.ManagedPIDForTest() != first {
		t.Fatal("compatible managed process was not reused")
	}
	if _, err = mgr.Ensure(ctx, "changed-node-key", cwd, false, cursorint.DiscardBootstrap{}); err != nil {
		t.Fatal(err)
	}
	if cursorint.ManagedPIDForTest() == first {
		t.Fatal("managed process with old credentials was reused")
	}
	expected, err := cursorint.ExpectedHealthForTest(root, cwd, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = cursorint.VerifySidecarForTest(ctx, port, expected, "changed-node-key"); err != nil {
		t.Fatal(err)
	}
}
