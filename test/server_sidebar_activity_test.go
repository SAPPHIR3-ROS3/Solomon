package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_sidebarOrderChangesOnSendOnly(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	createProject := func() string {
		t.Helper()
		response := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
		var created struct {
			Project struct{ ID string } `json:"project"`
		}
		decodeServerTestJSON(t, response, &created)
		return created.Project.ID
	}
	firstProject, secondProject := createProject(), createProject()
	created := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	first := &chatstore.Session{ID: chatstore.ChatIDHex("first", created), CreatedAt: created, LastUserMessageAt: created.Add(time.Hour)}
	second := &chatstore.Session{ID: chatstore.ChatIDHex("second", created), CreatedAt: created, LastUserMessageAt: created.Add(2 * time.Hour)}
	legacy := &chatstore.Session{ID: chatstore.ChatIDHex("legacy", created), CreatedAt: created, LastMessageAt: created.Add(5 * time.Hour), Messages: []chatstore.Message{
		{Role: "user", CreatedAt: created.Add(30 * time.Minute)},
		{Role: "assistant", CreatedAt: created.Add(5 * time.Hour)},
	}}
	write := func(projectID string, sess *chatstore.Session) {
		t.Helper()
		if err := chatstore.WriteSession(projectID, sess); err != nil {
			t.Fatal(err)
		}
	}
	write(firstProject, first)
	write(firstProject, legacy)
	write(secondProject, second)
	check := func(wantFirstProject string) {
		t.Helper()
		response, err := http.Get(server.URL + "/__solomon/projects")
		if err != nil {
			t.Fatal(err)
		}
		var sidebar struct {
			Projects []struct {
				ID    string
				Chats []struct {
					ID            string
					LastMessageAt time.Time
				}
			}
		}
		decodeServerTestJSON(t, response, &sidebar)
		if len(sidebar.Projects) != 2 || sidebar.Projects[0].ID != wantFirstProject {
			t.Fatalf("unexpected folder order: %+v", sidebar.Projects)
		}
		for _, project := range sidebar.Projects {
			if project.ID != firstProject {
				continue
			}
			if len(project.Chats) != 2 || project.Chats[0].ID != first.ID {
				t.Fatalf("assistant output reordered chats: %+v", project.Chats)
			}
			if !project.Chats[0].LastMessageAt.Equal(first.LastUserMessageAt) || !project.Chats[1].LastMessageAt.Equal(legacy.Messages[0].CreatedAt) {
				t.Fatalf("sidebar timestamps must reflect user sends: %+v", project.Chats)
			}
		}
	}
	check(secondProject)
	first.LastMessageAt = created.Add(3 * time.Hour)
	first.Messages = append(first.Messages, chatstore.Message{Role: "assistant", CreatedAt: first.LastMessageAt})
	write(firstProject, first)
	check(secondProject)
	first.LastUserMessageAt = created.Add(4 * time.Hour)
	write(firstProject, first)
	check(firstProject)
}
