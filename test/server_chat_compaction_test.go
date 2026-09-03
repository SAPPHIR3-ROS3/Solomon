package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_chatSnapshotCompactionKeepsHistoryInScrollablePayload(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	decodeServerTestJSON(t, projectResponse, &created)
	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	var createdChat struct {
		ID string `json:"id"`
	}
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	sess, err := chatstore.ReadSession(created.Project.ID, createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	separator := "---"
	summaryBody := separator + "\n[Conversation summary]\n" + separator + "\n\nKeep the API history visible.\n\n" + separator + "\n[Retained messages]\n" + separator + "\n\nUser:\nEarlier question\n\nAssistant:\nEarlier answer\n\n" + separator
	sess.Messages = []chatstore.Message{
		{Role: "assistant", Content: summaryBody},
		{Role: "user", Content: "Current question"},
		{Role: "assistant", Content: "Current answer"},
	}
	archived := make([]chatstore.Message, 0, 10)
	for index := 0; index < 10; index++ {
		role := "assistant"
		if index%2 == 0 {
			role = "user"
		}
		archived = append(archived, chatstore.Message{Role: role, Content: fmt.Sprintf("Archived message %d", index)})
	}
	sess.UncompactedRaw = []chatstore.UncompactedDump{{Messages: archived}}
	if err := chatstore.WriteSession(created.Project.ID, sess); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("open chat status = %d", response.StatusCode)
	}
	var opened struct {
		Messages []struct {
			Content          string `json:"content"`
			Kind             string `json:"kind"`
			RetainedMessages []struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"retainedMessages"`
			Summary string `json:"summary"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Messages) != 5 {
		t.Fatalf("messages = %d, want older archived messages, compaction, and current messages", len(opened.Messages))
	}
	if opened.Messages[0].Content != "Archived message 0" || opened.Messages[1].Content != "Archived message 1" {
		t.Fatalf("older archived messages not available before compaction: %+v", opened.Messages[:2])
	}
	compaction := opened.Messages[2]
	if compaction.Kind != "compaction" || compaction.Content != "" {
		t.Fatalf("unexpected compaction marker: %+v", compaction)
	}
	if compaction.Summary != "Keep the API history visible." {
		t.Fatalf("summary = %q", compaction.Summary)
	}
	if len(compaction.RetainedMessages) != 8 || compaction.RetainedMessages[0].Content != "Archived message 2" || compaction.RetainedMessages[7].Content != "Archived message 9" {
		t.Fatalf("retained messages = %+v", compaction.RetainedMessages)
	}
	if opened.Messages[3].Content != "Current question" || opened.Messages[4].Content != "Current answer" {
		t.Fatalf("current messages are not in the scrollable payload: %+v", opened.Messages)
	}
	for index, message := range opened.Messages {
		if index == 1 {
			continue
		}
		if strings.Contains(message.Content, "[Conversation summary]") {
			t.Fatal("summary was duplicated as a regular assistant message")
		}
	}
}
