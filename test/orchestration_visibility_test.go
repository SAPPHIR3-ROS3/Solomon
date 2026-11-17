package test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestOrchestrateSDKCallsRemainVisibleAfterFailureAndReload(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	response := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": root})
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
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	args, _ := json.Marshal(map[string]string{"intent": "inspect files", "source": `package main
import "sdk"
func main() {
 _, _ = sdk.ReadFile("missing.txt", "inspect missing file")
 _, _ = sdk.ReadFile("existing.txt", "inspect existing file")
}`})
	var starts []tooling.Invocation
	var finishes []string
	env := &agenttools.Env{
		ProjRoot: root, ParentToolCallID: "parent-call",
		DeferredToolStart: func(inv tooling.Invocation) { starts = append(starts, inv) },
		DeferredToolResult: func(inv tooling.Invocation, result json.RawMessage, err error) {
			if len(starts) != len(finishes)+1 || starts[len(starts)-1].ToolCallID != inv.ToolCallID {
				t.Error("tool result without matching start")
			}
			finishes = append(finishes, string(result))
			if len(finishes) == 1 && err == nil {
				t.Error("missing file did not fail")
			}
			if len(finishes) == 2 && err != nil {
				t.Errorf("existing file failed: %v", err)
			}
		},
	}
	result, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "orchestrate", Args: args, ToolCallID: "parent-call"})
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || len(finishes) != 2 {
		t.Fatalf("starts=%v finishes=%v result=%v", starts, finishes, result)
	}
	if starts[0].ToolCallID == starts[1].ToolCallID || !strings.Contains(finishes[0], "missing.txt") {
		t.Fatalf("incorrect child identities or error: %v %v", starts, finishes)
	}
	session, err := chatstore.ReadSession(project.Project.ID, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	session.Messages = []chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "parent-call", Name: "orchestrate", Arguments: string(args)}}},
		{Role: "tool", ToolCallID: "parent-call", Content: string(payload)},
	}
	if err := chatstore.WriteSession(project.Project.ID, session); err != nil {
		t.Fatal(err)
	}
	response, err = http.Get(server.URL + "/__solomon/projects/" + project.Project.ID + "/chats/" + chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	var opened struct {
		Messages []struct {
			ToolCalls []struct {
				ID, Name, Intent, Input, Status string
				Result                          map[string]any
			} `json:"toolCalls"`
		} `json:"messages"`
	}
	decodeServerTestJSON(t, response, &opened)
	if len(opened.Messages) == 0 || len(opened.Messages[0].ToolCalls) != 3 {
		t.Fatalf("missing SDK calls in reopened chat: %+v", opened)
	}
	calls := opened.Messages[0].ToolCalls
	if calls[1].ID != starts[0].ToolCallID || calls[1].Name != "readFile" || calls[1].Status != "error" || calls[1].Input != "missing.txt" || calls[1].Intent != "inspect missing file" {
		t.Fatalf("missing failed tool details: %+v", calls[1])
	}
	if calls[2].ID != starts[1].ToolCallID || calls[2].Status != "success" {
		t.Fatalf("missing successful tool details: %+v", calls[2])
	}
}
