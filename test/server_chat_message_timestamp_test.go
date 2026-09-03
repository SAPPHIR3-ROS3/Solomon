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
	sess.Messages = []chatstore.Message{{CreatedAt: want, Role: "user", Content: "hello"}}
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
		} `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Messages) != 1 {
		t.Fatalf("messages = %d", len(opened.Messages))
	}
	got, err := time.Parse(time.RFC3339Nano, opened.Messages[0].CreatedAt)
	if err != nil {
		t.Fatalf("createdAt = %q: %v", opened.Messages[0].CreatedAt, err)
	}
	if !got.Equal(want) {
		t.Fatalf("createdAt = %s, want %s", got, want)
	}
}
