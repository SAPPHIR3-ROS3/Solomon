package test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_chatMessageIncludesPersistedCreatedAt(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)
	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &chat)

	want := time.Date(2026, time.March, 12, 13, 26, 45, 123000000, time.FixedZone("test", 2*60*60))
	sess, err := chatstore.ReadSession(created.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	sess.Messages = []chatstore.Message{
		{CreatedAt: want, Role: "user", Content: "hello"},
		{Role: "assistant", Content: "working"},
		{Role: "user", Content: "internal tool correction", GUIHidden: true},
		{Role: "user", Content: "Your previous native API tool call was rejected. Every native tool call requires a non-empty string intent. Retry with a native Solomon tool_call whose JSON arguments match the declared schema and include a non-empty intent string. Do not emit XML, markdown pseudo-calls, or plain-text tool narration unless you mean to answer without tools."},
		{Role: "assistant", Content: "recovered"},
	}
	if err := chatstore.WriteSession(created.Project.ID, sess); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var opened struct {
		Messages []struct {
			CreatedAt string `json:"createdAt"`
			Role      string `json:"role"`
			Content   string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Messages) != 3 {
		t.Fatalf("messages = %d", len(opened.Messages))
	}
	if opened.Messages[0].Role != "user" || opened.Messages[1].Role != "assistant" || opened.Messages[2].Role != "assistant" || opened.Messages[2].Content != "recovered" {
		t.Fatalf("internal correction split the visible turn: %+v", opened.Messages)
	}
	got, err := time.Parse(time.RFC3339Nano, opened.Messages[0].CreatedAt)
	if err != nil {
		t.Fatalf("createdAt = %q: %v", opened.Messages[0].CreatedAt, err)
	}
	if !got.Equal(want) {
		t.Fatalf("createdAt = %s, want %s", got, want)
	}
}
