package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerSubagentHierarchyAndRecursiveSubchats(t *testing.T) {
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
	messages := func(depth int) []chatstore.Message {
		id := fmt.Sprintf("call-%d", depth)
		return []chatstore.Message{
			{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: id, Name: "subagent", Arguments: `{"intent":"inspect nested task","task":"inspect"}`}}},
			{Role: "tool", ToolCallID: id, Content: fmt.Sprintf(`{"ok":true,"status":"done","subchatId":"sub-%d"}`, depth)},
		}
	}
	session, err := chatstore.ReadSession(project.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	session.Messages = messages(1)
	if err := chatstore.WriteSession(project.Project.ID, session); err != nil {
		t.Fatal(err)
	}
	for depth := 1; depth <= 4; depth++ {
		child := &chatstore.SubSession{ID: fmt.Sprintf("sub-%d", depth), Depth: depth, Origin: chatstore.SubOriginParent, ProjectHex: project.Project.ID, ParentChatID: chat.ID, ParentToolCallID: fmt.Sprintf("call-%d", depth), Status: chatstore.SubStatusDone}
		if depth > 1 {
			child.ParentSubchatID = fmt.Sprintf("sub-%d", depth-1)
		}
		if depth < 4 {
			child.Messages = messages(depth + 1)
		}
		if err := chatstore.WriteSubSession(project.Project.ID, child); err != nil {
			t.Fatal(err)
		}
	}
	for _, legacy := range []bool{false, true} {
		if legacy {
			sessions, err := chatstore.ListSubSessions(project.Project.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, child := range sessions {
				child.Depth = 0
				child.ParentSubchatID = ""
				if err := chatstore.WriteSubSession(project.Project.ID, child); err != nil {
					t.Fatal(err)
				}
			}
		}
		for level := 0; level < 4; level++ {
			endpoint := "/chats/" + chat.ID
			if level > 0 {
				endpoint = fmt.Sprintf("/subchats/sub-%d", level)
			}
			t.Run(fmt.Sprintf("legacy=%v/level=%d", legacy, level), func(t *testing.T) {
				res, err := http.Get(server.URL + "/__solomon/projects/" + project.Project.ID + endpoint)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				var snapshot struct {
					Messages []struct {
						ToolCalls []struct {
							ID     string         `json:"id"`
							Depth  int            `json:"subagentDepth"`
							Parent string         `json:"parentSubchatId"`
							Result map[string]any `json:"result"`
						} `json:"toolCalls"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(res.Body).Decode(&snapshot); err != nil {
					t.Fatal(err)
				}
				if len(snapshot.Messages) == 0 || len(snapshot.Messages[0].ToolCalls) != 4-level {
					t.Fatalf("missing descendants: %+v", snapshot)
				}
				for index, tool := range snapshot.Messages[0].ToolCalls {
					depth := level + index + 1
					parent := ""
					if depth > 1 {
						parent = fmt.Sprintf("sub-%d", depth-1)
					}
					if tool.Depth != depth || tool.Parent != parent || tool.Result["subchatId"] != fmt.Sprintf("sub-%d", depth) {
						t.Fatalf("wrong hierarchy at depth %d: %+v", depth, tool)
					}
				}
			})
		}
	}
}
