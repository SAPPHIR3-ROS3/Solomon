package test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_activeAgentsTreeIncludesNestedSubagents(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	now := time.Now().UTC()
	parent := &chatstore.SubSession{
		ID:               "sub-parent",
		Title:            "Reviewer",
		CreatedAt:        now,
		LastMessageAt:    now,
		Origin:           chatstore.SubOriginParent,
		ProjectHex:       created.Project.ID,
		ParentChatID:     createdChat.ID,
		ParentToolCallID: "call-parent",
		Status:           chatstore.SubStatusRunning,
		Messages:         []chatstore.Message{{Role: "user", Content: "review the diff"}},
	}
	child := &chatstore.SubSession{
		ID:               "sub-child",
		Title:            "Searcher",
		CreatedAt:        now,
		LastMessageAt:    now,
		Origin:           chatstore.SubOriginParent,
		ProjectHex:       created.Project.ID,
		ParentChatID:     parent.ID,
		ParentToolCallID: "call-child",
		Status:           chatstore.SubStatusQueued,
		Messages:         []chatstore.Message{{Role: "user", Content: "search files"}},
	}
	done := &chatstore.SubSession{
		ID:            "sub-done",
		Title:         "Finished",
		CreatedAt:     now,
		LastMessageAt: now,
		Origin:        chatstore.SubOriginParent,
		ProjectHex:    created.Project.ID,
		ParentChatID:  createdChat.ID,
		Status:        chatstore.SubStatusDone,
	}
	paused := &chatstore.SubSession{
		ID:            "sub-paused",
		Title:         "Paused",
		CreatedAt:     now,
		LastMessageAt: now,
		Origin:        chatstore.SubOriginParent,
		ProjectHex:    created.Project.ID,
		ParentChatID:  createdChat.ID,
		Status:        chatstore.SubStatusPaused,
	}
	if err := chatstore.WriteSubSession(created.Project.ID, parent); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.WriteSubSession(created.Project.ID, child); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.WriteSubSession(created.Project.ID, done); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.WriteSubSession(created.Project.ID, paused); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/__solomon/active-agents")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var payload struct {
		Projects []struct {
			ID     string `json:"id"`
			Agents []struct {
				Kind     string `json:"kind"`
				ID       string `json:"id"`
				Children []struct {
					Kind             string `json:"kind"`
					ID               string `json:"id"`
					RootChatID       string `json:"rootChatID"`
					ParentToolCallID string `json:"parentToolCallID"`
					Children         []struct {
						ID         string `json:"id"`
						Kind       string `json:"kind"`
						RootChatID string `json:"rootChatID"`
					} `json:"children"`
				} `json:"children"`
			} `json:"agents"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].ID != created.Project.ID {
		t.Fatalf("projects = %+v", payload.Projects)
	}
	if len(payload.Projects[0].Agents) != 1 {
		t.Fatalf("agents = %+v", payload.Projects[0].Agents)
	}
	agent := payload.Projects[0].Agents[0]
	if agent.Kind != "chat" || agent.ID != createdChat.ID {
		t.Fatalf("root agent = %+v", agent)
	}
	if len(agent.Children) != 1 || agent.Children[0].ID != "sub-parent" {
		t.Fatalf("children = %+v", agent.Children)
	}
	if agent.Children[0].RootChatID != createdChat.ID || agent.Children[0].ParentToolCallID != "call-parent" {
		t.Fatalf("parent subagent = %+v", agent.Children[0])
	}
	if len(agent.Children[0].Children) != 1 || agent.Children[0].Children[0].ID != "sub-child" {
		t.Fatalf("nested = %+v", agent.Children[0].Children)
	}
	if agent.Children[0].Children[0].RootChatID != createdChat.ID {
		t.Fatalf("nested root = %+v", agent.Children[0].Children[0])
	}
}

func TestServerActiveAgentsRetainCallerHierarchy(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	response := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
	var project struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, response, &project)
	response = postJSONForServerTest(t, server.URL+"/__solomon/projects/"+project.Project.ID+"/chats", map[string]string{})
	var chat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, response, &chat)
	for _, status := range []string{chatstore.SubStatusRunning, chatstore.SubStatusDone} {
		t.Run(status, func(t *testing.T) {
			parent := &chatstore.SubSession{ID: "caller", Title: "Caller", Depth: 1, Status: status, Origin: chatstore.SubOriginParent, ProjectHex: project.Project.ID, ParentChatID: chat.ID}
			child := &chatstore.SubSession{ID: "child", Title: "Child", Depth: 2, Status: chatstore.SubStatusRunning, Origin: chatstore.SubOriginParent, ProjectHex: project.Project.ID, ParentChatID: chat.ID, ParentSubchatID: parent.ID}
			for _, sub := range []*chatstore.SubSession{parent, child} {
				if err := chatstore.WriteSubSession(project.Project.ID, sub); err != nil {
					t.Fatal(err)
				}
			}
			res, err := http.Get(server.URL + "/__solomon/active-agents")
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			type node struct {
				ID       string            `json:"id"`
				Depth    int               `json:"subagentDepth"`
				Parent   string            `json:"parentSubchatId"`
				Status   string            `json:"status"`
				Children []json.RawMessage `json:"children"`
			}
			var payload struct {
				Projects []struct {
					Agents []node `json:"agents"`
				} `json:"projects"`
			}
			if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Projects) != 1 || len(payload.Projects[0].Agents) != 1 {
				t.Fatalf("unexpected roots: %+v", payload)
			}
			root := payload.Projects[0].Agents[0]
			if root.ID != chat.ID || len(root.Children) != 1 {
				t.Fatalf("caller missing: %+v", root)
			}
			var caller, descendant node
			if err := json.Unmarshal(root.Children[0], &caller); err != nil {
				t.Fatal(err)
			}
			if caller.ID != parent.ID || caller.Status != status || caller.Depth != 1 || len(caller.Children) != 1 {
				t.Fatalf("wrong caller: %+v", caller)
			}
			if err := json.Unmarshal(caller.Children[0], &descendant); err != nil {
				t.Fatal(err)
			}
			if descendant.ID != child.ID || descendant.Parent != parent.ID || descendant.Depth != 2 {
				t.Fatalf("child flattened: %+v", descendant)
			}
		})
	}
}
