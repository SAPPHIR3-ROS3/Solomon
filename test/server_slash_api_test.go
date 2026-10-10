package test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func TestServerRuntime_GUISlashCommands(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	registry := skills.NewRegistry()
	registry.Global["gui-test"] = skills.SkillEntry{Name: "gui-test"}
	if err := skills.SaveRegistry(registryPath, registry); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/__solomon/slash-commands?query=gui-test", "/__solomon/slash-commands?query=terminal"} {
		response, err := http.Get(server.URL + route)
		if err != nil {
			t.Fatal(err)
		}
		var suggestions []struct {
			Tag string `json:"tag"`
		}
		decodeServerTestJSON(t, response, &suggestions)
		if strings.Contains(route, "gui-test") && (len(suggestions) != 1 || suggestions[0].Tag != "/gui-test") {
			t.Fatalf("skill suggestions: %+v", suggestions)
		}
		if strings.Contains(route, "terminal") && len(suggestions) != 0 {
			t.Fatalf("terminal suggestions: %+v", suggestions)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "example.go"), []byte("package example"), 0600); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": root}), &created)
	base := server.URL + "/__solomon/projects/" + created.Project.ID
	response, err := http.Get(base + "/slash-commands?query=he")
	if err != nil {
		t.Fatal(err)
	}
	var suggestions []struct {
		Tag         string `json:"tag"`
		Description string `json:"description"`
	}
	decodeServerTestJSON(t, response, &suggestions)
	if len(suggestions) != 1 || suggestions[0].Tag != "/help" || suggestions[0].Description == "" {
		t.Fatalf("suggestions: %+v", suggestions)
	}
	response, err = http.Get(base + "/at-mentions?query=example")
	if err != nil {
		t.Fatal(err)
	}
	var mentions []struct {
		Path string `json:"path"`
		Tag  string `json:"tag"`
	}
	decodeServerTestJSON(t, response, &mentions)
	if len(mentions) != 1 || mentions[0].Path != "example.go" || !strings.HasPrefix(mentions[0].Tag, "@") {
		t.Fatalf("mentions: %+v", mentions)
	}
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, base+"/chats", map[string]string{}), &chat)
	for _, command := range []string{"/help", "/not-a-real-command", "/terminal"} {
		response = postJSONForServerTest(t, base+"/chats/"+chat.ID+"/messages", map[string]string{"content": command})
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d, %s, %v", command, response.StatusCode, body, err)
		}
		if !strings.Contains(string(body), "chat_snapshot") {
			t.Fatalf("no snapshot: %s", body)
		}
	}
	sess, err := chatstore.ReadSession(created.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 6 {
		encoded, _ := json.Marshal(sess.Messages)
		t.Fatalf("messages: %s", encoded)
	}
	for i, expected := range []string{"/help", "unknown command", "requires the terminal"} {
		if !strings.Contains(sess.Messages[i*2+1].Content, expected) {
			t.Fatalf("output: %s", sess.Messages[i*2+1].Content)
		}
		if strings.Contains(sess.Messages[i*2+1].Content, "\x1b") {
			t.Fatal("ANSI in GUI output")
		}
	}
}
