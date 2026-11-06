package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestSubagentFailurePreservesSubchatMetadata(t *testing.T) {
	env := &agenttools.Env{
		CurrentMode: func() string { return "agent" },
		SetMode: func(string) {},
		RunSubagent: func(context.Context, agenttools.SubagentRequest) (agenttools.SubagentResponse, error) {
			return agenttools.SubagentResponse{SubchatID: "child", Status: "paused", Output: "Partial progress"}, fmt.Errorf("provider failed")
		},
	}
	result, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "subagent",
		Args: json.RawMessage("{\"task\":\"Verify the failure\",\"intent\":\"Test subagent failure metadata\"}"),
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != false || payload["subchatId"] != "child" || payload["status"] != "paused" || payload["output"] != "Partial progress" || payload["error"] != "provider failed" {
		t.Fatalf("lost failed subchat metadata: %s", encoded)
	}
}
