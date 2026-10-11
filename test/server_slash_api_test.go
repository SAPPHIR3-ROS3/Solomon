package test

import (
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
	// Running a command from Home must not allocate a session or sidebar thread.
	response = postJSONForServerTest(t, base+"/commands", map[string]string{"content": "/help"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("project command: %d", response.StatusCode)
	}
	response.Body.Close()
	beforeChats, err := chatstore.ListRecent(created.Project.ID, 100)
	if err != nil || len(beforeChats) != 0 {
		t.Fatalf("command created threads: %+v, %v", beforeChats, err)
	}
	projectDir, err := paths.ProjectRoot(created.Project.ID)
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := os.ReadFile(filepath.Join(projectDir, "commands.jsonl"))
	if err != nil || !strings.Contains(string(transcript), `"content":"/help"`) {
		t.Fatalf("missing command transcript: %s, %v", transcript, err)
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
		t.Fatalf("command input and result must remain in the transcript: %+v", sess.Messages)
	}
	for index, command := range []string{"/help", "/not-a-real-command", "/terminal"} {
		if sess.Messages[index*2].Role != "user" || sess.Messages[index*2].Content != command {
			t.Fatalf("unexpected command transcript: %+v", sess.Messages)
		}
	}
	response, err = http.Get(base + "/chats/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	var displayed struct {
		Messages []any `json:"messages"`
	}
	decodeServerTestJSON(t, response, &displayed)
	if len(displayed.Messages) != 0 {
		t.Fatalf("commands leaked into the visible chat: %+v", displayed.Messages)
	}
	response, err = http.Get(server.URL + "/__solomon/notifications")
	if err != nil {
		t.Fatal(err)
	}
	var notifications []struct {
		Message string `json:"message"`
	}
	decodeServerTestJSON(t, response, &notifications)
	for _, expected := range []string{"/help", "unknown command", "requires the terminal"} {
		found := false
		for _, notice := range notifications {
			if strings.Contains(notice.Message, expected) {
				found = true
			}
			if strings.Contains(notice.Message, "\x1b") {
				t.Fatal("ANSI in notification")
			}
		}
		if !found {
			t.Fatalf("missing notification %q: %+v", expected, notifications)
		}
	}
}
