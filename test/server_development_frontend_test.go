package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestExportGUISourcesPreservesCustomization(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	directory, err := serverruntime.ExportGUISourcesForTest()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"package-lock.json", "src/main.tsx", "vite.config.ts", "scripts/dev.mjs", "wailsjs/runtime/runtime.js", "icon.svg"} {
		if _, err := os.Stat(filepath.Join(directory, path)); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	custom := filepath.Join(directory, "src", "main.tsx")
	if err := os.WriteFile(custom, []byte("customized"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := serverruntime.ExportGUISourcesForTest(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(custom)
	if string(data) != "customized" {
		t.Fatal("existing customization overwritten")
	}
}

func TestDevelopmentSwitchKeepsDaemonAndRequestsAlive(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required")
	}
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("user_name='Fixture'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	directory, err := serverruntime.ExportGUISourcesForTest()
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(directory, "node_modules", "vite", "bin", "vite.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	// An actual Node HTTP process exercises readiness, proxying and tree cleanup
	// without downloading dependencies or touching the running installation.
	if err := os.WriteFile(entry, []byte(`const http=require('http');const port=Number(process.argv[process.argv.indexOf('--port')+1]);http.createServer((req,res)=>res.end('custom frontend')).listen(port,'127.0.0.1');`), 0600); err != nil {
		t.Fatal(err)
	}
	initial := serverruntime.State{PID: os.Getpid(), Mode: "normal", Vite: "stopped", StartedAt: time.Now().UTC(), URL: "http://127.0.0.1:12345"}
	started, release := make(chan struct{}), make(chan struct{})
	d := serverruntime.NewDevelopmentFrontendForTest(initial, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "original request completed")
	}))
	defer d.Close()
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { d.ServeHTTP(response, httptest.NewRequest("GET", "/", nil)); close(done) }()
	<-started
	next, err := d.Change(context.Background(), "dev", directory)
	close(release)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if response.Body.String() != "original request completed" {
		t.Fatal("in-flight frontend request interrupted")
	}
	if next.PID != initial.PID || next.URL != initial.URL || !next.StartedAt.Equal(initial.StartedAt) {
		t.Fatal("daemon identity changed")
	}
	if next.Vite != "running" || next.DevDir != directory {
		t.Fatalf("invalid dev state: %+v", next)
	}
	recorder := httptest.NewRecorder()
	d.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Body.String() != "custom frontend" {
		t.Fatalf("frontend was not switched: %s", recorder.Body.String())
	}
	// Repeated setup reuses the process and edited sources.
	again, err := d.Change(context.Background(), "dev", directory)
	if err != nil || again.VitePID != next.VitePID {
		t.Fatalf("setup was not idempotent: %+v %v", again, err)
	}
	invalid := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalid, "existing.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Change(context.Background(), "dev", invalid); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	if d.Snapshot().VitePID != next.VitePID {
		t.Fatal("failed transition replaced working frontend")
	}
	mode, source, err := config.ResolveServerDevelopment("normal", "")
	if err != nil || mode != "dev" || source != directory {
		t.Fatalf("dev preference not persisted: %s %s %v", mode, source, err)
	}
	if _, err := d.Change(context.Background(), "normal", ""); err != nil {
		t.Fatal(err)
	}
	mode, _, err = config.ResolveServerDevelopment("dev", directory)
	if err != nil || mode != "normal" {
		t.Fatalf("normal preference not persisted: %s %v", mode, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "src")); err != nil {
		t.Fatal("customization deleted")
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	if response, err := client.Get(next.ViteURL); err == nil {
		response.Body.Close()
		t.Fatal("old Vite process still running")
	}
}

func TestPrepareGUIWorkspaceInitializesOnlyEmptyFolders(t *testing.T) {
	for _, existing := range []bool{true, false} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			parent := t.TempDir()
			selected := filepath.Join(parent, "selected")
			if existing {
				if err := os.Mkdir(selected, 0700); err != nil {
					t.Fatal(err)
				}
			}
			directory, err := serverruntime.PrepareGUIWorkspace(selected)
			if err != nil {
				t.Fatal(err)
			}
			if directory != filepath.Join(selected, "gui") {
				t.Fatalf("source directory: %s", directory)
			}
			for _, path := range []string{"gui/package.json", "gui/src/main.tsx", "docs/README.md", "internal/logo/logo.txt"} {
				if _, err := os.Stat(filepath.Join(selected, filepath.FromSlash(path))); err != nil {
					t.Fatal(err)
				}
			}
			entries, _ := os.ReadDir(parent)
			if len(entries) != 1 {
				t.Fatal("files exported outside selected folder")
			}
			if again, err := serverruntime.PrepareGUIWorkspace(selected); err != nil || again != directory {
				t.Fatalf("existing base not reused: %s %v", again, err)
			}
		})
	}
	selected := t.TempDir()
	file := filepath.Join(selected, "notes.txt")
	if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := serverruntime.PrepareGUIWorkspace(selected); err == nil {
		t.Fatal("unrelated nonempty folder accepted")
	}
	entries, _ := os.ReadDir(selected)
	data, _ := os.ReadFile(file)
	if len(entries) != 1 || string(data) != "preserve" {
		t.Fatal("existing files modified")
	}
}

func TestDevelopmentRealExportedGUI(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_REAL_GUI") != "1" {
		t.Skip("opt-in npm installation and actual Vite startup")
	}
	t.Setenv("SOLOMON_HOME", t.TempDir())
	directory, err := serverruntime.ExportGUISourcesForTest()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := serverruntime.PrepareGUIDependencies(ctx, directory); err != nil {
		t.Fatal(err)
	}
	address, cmd, err := serverruntime.StartViteForTest(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer serverruntime.StopViteForTest(cmd)
	client := &http.Client{Timeout: 30 * time.Second}
	for _, path := range []string{"/", "/icon.svg", "/src/main.tsx", "/src/App.tsx", "/src/home/Welcome.tsx", "/src/settings/DocsViewer.tsx"} {
		response, err := client.Get(address.String() + path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("exported GUI %s: %s %s %v", path, response.Status, data, err)
		}
	}
}

func TestDevelopmentDependencyFailurePreservesInterface(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	directory, err := serverruntime.ExportGUISourcesForTest()
	if err != nil {
		t.Fatal(err)
	}
	d := serverruntime.NewDevelopmentFrontendForTest(serverruntime.State{Mode: "normal"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "working interface") }))
	if _, err := d.Change(context.Background(), "dev", directory); err == nil || !strings.Contains(err.Error(), "Node.js") {
		t.Fatalf("missing prerequisite: %v", err)
	}
	response := httptest.NewRecorder()
	d.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	if response.Body.String() != "working interface" || d.Snapshot().Mode != "normal" {
		t.Fatal("preparation failure broke active interface")
	}
	mode, _, err := config.ResolveServerDevelopment("normal", "")
	if err != nil || mode != "normal" {
		t.Fatal("failed preparation changed saved startup mode")
	}
}
