package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestExecWritesCorrelatedToolCallLifecycleLog(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	logDir := t.TempDir()
	if err := logging.Configure(logging.Config{Dir: logDir, WriteFile: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = logging.Configure(logging.Config{WriteConsole: true, WriteFile: false})
	})

	_, err := agenttools.Exec(context.Background(), &agenttools.Env{
		ParentToolCallID:   "orchestrate-call-1",
		ParentToolName:     "orchestrate",
		AllowDeferredTools: true,
	}, "agent", tooling.Invocation{
		Name: "unknownDeferredTool",
		Args: json.RawMessage(`{"intent":"exercise tool call logging","token":"must not be logged"}`),
	})
	if err == nil {
		t.Fatal("expected unknown tool error")
	}

	lines, err := logging.ReadLast(20)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"tool call started",
		"tool call failed",
		"unknownDeferredTool",
		"parent_tool:orchestrate",
		"parent_tool_call_id:orchestrate-call-1",
		"deferred:true",
		"exercise tool call logging",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("log does not contain %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "must not be logged") {
		t.Fatalf("raw tool arguments leaked into log:\n%s", joined)
	}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "nested.txt"), []byte("nested payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	source := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	content, err := sdk.ReadFile("nested.txt", "nested read")
	if err != nil {
		panic(err)
	}
	fmt.Print(content)
}`
	orchestrateArgs, err := json.Marshal(map[string]string{
		"source": source,
		"intent": "exercise nested tool logging",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := agenttools.Exec(context.Background(), &agenttools.Env{
		ProjRoot:         project,
		ParentToolCallID: "orchestrate-call-2",
	}, "agent", tooling.Invocation{
		Name:       "orchestrate",
		Args:       orchestrateArgs,
		ToolCallID: "orchestrate-call-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resultMap, ok := result.(map[string]any); !ok || resultMap["ok"] != true {
		t.Fatalf("orchestrate result=%v", result)
	}

	lines, err = logging.ReadLast(40)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(lines, "\n")
	for _, want := range []string{
		"tool:orchestrate",
		"tool:readFile",
		"parent_tool:orchestrate",
		"parent_tool_call_id:orchestrate-call-2",
		"nested read",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("nested lifecycle log does not contain %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "nested.txt") || strings.Contains(joined, "nested payload") {
		t.Fatalf("nested raw tool arguments leaked into log:\n%s", joined)
	}
}
