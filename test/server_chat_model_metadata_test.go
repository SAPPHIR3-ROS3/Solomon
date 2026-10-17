package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestMessageStatsPreserveRecordedTarget(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	var created struct {
		Project struct{ ID string } `json:"project"`
	}
	decodeServerTestJSON(t, postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()}), &created)
	chatURL := server.URL + "/__solomon/projects/" + created.Project.ID + "/chats"
	var chat struct{ ID string }
	decodeServerTestJSON(t, postJSONForServerTest(t, chatURL, map[string]string{}), &chat)
	session, err := chatstore.ReadSession(created.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	session.Messages = []chatstore.Message{
		{Role: "assistant", Content: "hello", Provider: "custom-provider", Model: "recorded-model"},
		{Role: "assistant", Content: "legacy answer", ResponseTokens: 5},
	}
	if err := chatstore.WriteSession(created.Project.ID, session); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(chatURL + "/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var opened struct {
		Messages []struct {
			Stats *struct {
				Provider       string `json:"provider"`
				Model          string `json:"model"`
				ResponseTokens int64  `json:"responseTokens"`
			} `json:"stats"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Messages) != 2 || opened.Messages[0].Stats == nil || opened.Messages[1].Stats == nil {
		t.Fatalf("missing message statistics: %+v", opened.Messages)
	}
	stats := opened.Messages[0].Stats
	if stats.Provider != "custom-provider" || stats.Model != "recorded-model" {
		t.Fatalf("stats lost recorded target: %+v", stats)
	}
	legacy := opened.Messages[1].Stats
	if legacy.Provider != "" || legacy.Model != "" || legacy.ResponseTokens != 5 {
		t.Fatalf("legacy stats: %+v", legacy)
	}
}
