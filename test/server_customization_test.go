package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	servercli "github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestServerRuntime_customizationToolKeepsDaemonAlive(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required")
	}
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	if err := os.WriteFile(filepath.Join(server.Home, "config.toml"), []byte("user_name='Fixture'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	entry := filepath.Join(directory, "node_modules", "vite", "bin", "vite.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`const http=require('http');const port=Number(process.argv[process.argv.indexOf('--port')+1]);http.createServer((req,res)=>res.end('personalized')).listen(port,'127.0.0.1');`), 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(mode, action, directory string) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"action": action, "directory": directory, "intent": "Personalize the requested interface"})
		result, err := agenttools.Exec(context.Background(), &agenttools.Env{Cfg: config.EmptyRoot()}, mode, tooling.Invocation{Name: "customizeUI", Args: raw})
		if err != nil {
			t.Fatal(err)
		}
		return result.(map[string]any)
	}
	if result := invoke("chat", "status", ""); result["mode"] != "normal" {
		t.Fatal(result)
	}
	if result := invoke("agent", "dev", directory); result["restart_required"] != false || result["dev_directory"] != directory {
		t.Fatal(result)
	}
	health := getHealthForTest(t, server.URL)
	if health.Server.Mode != "dev" || health.Server.PID != server.PID || !health.Server.StartedAt.Equal(server.StartedAt) {
		t.Fatalf("frontend transition restarted daemon: %+v", health)
	}
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(data) != "personalized" {
		t.Fatalf("frontend was not replaced: %s %v", data, err)
	}
	if err := servercli.Run([]string{"start", "normal"}); err != nil {
		t.Fatal(err)
	}
	health = getHealthForTest(t, server.URL)
	if health.Server.Mode != "normal" || health.Server.Vite != "stopped" || health.Server.PID != server.PID {
		t.Fatal(health)
	}
	// The directory remains selected when returning to the custom interface.
	if result := invoke("agent", "dev", ""); result["dev_directory"] != directory {
		t.Fatal(result)
	}
	invoke("agent", "normal", "")
}
