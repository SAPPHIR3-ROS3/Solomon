package test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
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

// In a chat, the typed skill line is the request of the agent turn, so it stays visible.
func TestServerRuntime_ChatSkillCommandKeepsRequestVisible(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("---\nname: teach\ndescription: Teach.\n---\nTeach the user.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := skills.NewRegistry()
	registry.Global["teach"] = skills.SkillEntry{Name: "teach", SkillMdPath: skillPath}
	if err := skills.SaveRegistry(registryPath, registry); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()}), &created)
	base := server.URL + "/__solomon/projects/" + created.Project.ID + "/chats"
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, base, map[string]string{}), &chat)
	for _, content := range []string{"/teach review cryptography", "/help"} {
		response := postJSONForServerTest(t, base+"/"+chat.ID+"/messages", map[string]string{"content": content})
		stream, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		// The GUI does not show slash lines optimistically, so chat_start carries the skill request.
		sentUser := strings.Contains(string(stream), `"stream-user-`)
		if sentUser != (content != "/help") {
			t.Fatalf("%s: chat_start user sent=%v", content, sentUser)
		}
	}
	response, err := http.Get(base + "/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	var served struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	decodeServerTestJSON(t, response, &served)
	var shown []string
	for _, message := range served.Messages {
		if message.Role == "user" {
			shown = append(shown, message.Content)
		}
	}
	if len(shown) != 1 || shown[0] != "/teach review cryptography" {
		t.Fatalf("chat API should return only the skill request, got %q", shown)
	}
	sess, err := chatstore.ReadSession(created.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The chat is named after the request, not the skill command or the default title.
	if title := strings.ToLower(sess.Title); !strings.Contains(title, "cryptography") || strings.Contains(title, "teach") {
		t.Fatalf("skill chat title = %q", sess.Title)
	}
	hidden := map[string]bool{}
	for _, message := range sess.Messages {
		if message.Role == "user" {
			hidden[message.Content] = message.GUIHidden
		}
	}
	if got, ok := hidden["/teach review cryptography"]; !ok || got {
		t.Fatalf("skill request must be recorded and visible: recorded=%v hidden=%v", ok, got)
	}
	if !hidden["/help"] {
		t.Fatal("builtin command line should stay hidden")
	}
}

// A bare skill command in the GUI shows only the typed line; the expanded skill body reaches the model only.
func TestServerRuntime_BareSkillCommandKeepsSkillBodyModelFacing(t *testing.T) {
	sse := anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "ok"}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
	)
	var mu sync.Mutex
	var sentToModel []string
	provider := newAnthropicMockServer(t, "/v1/messages", func(r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		sentToModel = append(sentToModel, string(body))
		mu.Unlock()
	}, sse, 0)
	defer provider.Close()
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	home := os.Getenv("SOLOMON_HOME")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(fmt.Sprintf(`[current]
provider = "test"
model = "claude-test"

[providers.test]
base_url = %q
api_key = "test-key"
api_protocol = "anthropic"
`, provider.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	const skillBody = "Teach the user step by step."
	skillPath := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("---\nname: teach\ndescription: Teach.\n---\n"+skillBody+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	registry := skills.NewRegistry()
	registry.Global["teach"] = skills.SkillEntry{Name: "teach", SkillMdPath: skillPath}
	if err := skills.SaveRegistry(registryPath, registry); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()}), &created)
	base := server.URL + "/__solomon/projects/" + created.Project.ID + "/chats"
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, base, map[string]string{}), &chat)

	response := postJSONForServerTest(t, base+"/"+chat.ID+"/messages", map[string]string{"content": "/teach"})
	stream, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Live stream: the command is shown and the snapshot arrives, but the skill body never reaches the GUI.
	if !strings.Contains(string(stream), `"stream-user-`) || !strings.Contains(string(stream), `"type":"chat_snapshot"`) {
		t.Fatalf("stream lacks the visible command or the final snapshot: %s", stream)
	}
	if strings.Contains(string(stream), skillBody) {
		t.Fatalf("stream exposed the skill body: %s", stream)
	}
	mu.Lock()
	reachedModel := false
	for _, body := range sentToModel {
		reachedModel = reachedModel || strings.Contains(body, skillBody)
	}
	mu.Unlock()
	if !reachedModel {
		t.Fatal("the model never received the skill body")
	}

	// Reopened chat: one visible user message, the typed command, with the skill body kept as API content.
	reopened, err := http.Get(base + "/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	var served struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	decodeServerTestJSON(t, reopened, &served)
	var shown []string
	for _, message := range served.Messages {
		if message.Role == "user" {
			shown = append(shown, message.Content)
		}
	}
	if len(shown) != 1 || shown[0] != "/teach" {
		t.Fatalf("reopened chat user messages = %q", shown)
	}
	sess, err := chatstore.ReadSession(created.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	users := 0
	for _, message := range sess.Messages {
		if message.Role != "user" {
			continue
		}
		users++
		if message.Content != "/teach" || message.GUIHidden || !strings.Contains(message.APIContent, skillBody) {
			t.Fatalf("persisted user message: content=%q hidden=%v api has body=%v", message.Content, message.GUIHidden, strings.Contains(message.APIContent, skillBody))
		}
	}
	if users != 1 {
		t.Fatalf("persisted %d user messages, want 1", users)
	}
}
