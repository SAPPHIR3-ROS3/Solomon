package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerSubagentCompletionUpdatesActivityStatus(t *testing.T) {
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
	session, err := chatstore.ReadSession(project.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	session.Messages = []chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "spawn", Name: "subagent", Arguments: `{"intent":"inspect","task":"inspect","run_in_background":true}`}}},
		{Role: "tool", ToolCallID: "spawn", Content: `{"ok":true,"subchatId":"child","status":"running"}`},
	}
	if err := chatstore.WriteSession(project.Project.ID, session); err != nil {
		t.Fatal(err)
	}
	child := &chatstore.SubSession{ID: "child", Origin: chatstore.SubOriginParent, ProjectHex: project.Project.ID, ParentChatID: chat.ID, ParentToolCallID: "spawn"}
	for _, state := range []struct{ persisted, activity string }{{"running", "running"}, {"done", "success"}, {"paused", "error"}, {"cancelled", "error"}} {
		t.Run(state.persisted, func(t *testing.T) {
			child.Status = state.persisted
			if err := chatstore.WriteSubSession(project.Project.ID, child); err != nil {
				t.Fatal(err)
			}
			res, err := http.Get(server.URL + "/__solomon/projects/" + project.Project.ID + "/chats/" + chat.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			var snapshot struct {
				Messages []struct {
					ToolCalls []struct {
						Status string         `json:"status"`
						Result map[string]any `json:"result"`
					} `json:"toolCalls"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(res.Body).Decode(&snapshot); err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Messages) == 0 || len(snapshot.Messages[0].ToolCalls) != 1 {
				t.Fatalf("unexpected snapshot: %+v", snapshot)
			}
			tool := snapshot.Messages[0].ToolCalls[0]
			if tool.Status != state.activity || tool.Result["subagentStatus"] != state.persisted {
				t.Fatalf("stale activity: %+v", tool)
			}
		})
	}
}
