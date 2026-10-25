package test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

// A skill started from Home runs an agent turn, so it must go to a chat instead of the hidden command transcript.
func TestServerRuntime_HomeSkillCommandOpensChat(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	registry := skills.NewRegistry()
	registry.Global["teach"] = skills.SkillEntry{Name: "teach"}
	if err := skills.SaveRegistry(registryPath, registry); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()}), &created)
	base := server.URL + "/__solomon/projects/" + created.Project.ID
	for _, content := range []string{"/teach review cryptography", "/skill:teach"} {
		response := postJSONForServerTest(t, base+"/commands", map[string]string{"content": content})
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", content, response.StatusCode)
		}
		var payload struct {
			OpenChat bool `json:"open_chat"`
		}
		decodeServerTestJSON(t, response, &payload)
		if !payload.OpenChat {
			t.Fatalf("%s: skill command did not ask for a chat", content)
		}
	}
	response := postJSONForServerTest(t, base+"/commands", map[string]string{"content": "/help"})
	var payload struct {
		OpenChat bool `json:"open_chat"`
	}
	decodeServerTestJSON(t, response, &payload)
	if payload.OpenChat {
		t.Fatal("/help is a builtin and must run on Home")
	}
	projectDir, err := paths.ProjectRoot(created.Project.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, "commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.TrimSpace(string(data)), "\n") + 1; n != 2 {
		t.Fatalf("only /help should reach the command transcript, got %d lines", n)
	}
}
