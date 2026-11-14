package test

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestSubagentSetupFailureClearsRunningStatus(t *testing.T) {
	for _, background := range []bool{false, true} {
		name := "synchronous"
		if background {
			name = "background"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("SOLOMON_HOME", t.TempDir())
			cfg := &config.Root{Current: config.Current{Provider: "test", Model: "test-model"}}
			r := agentruntime.NewTestRuntime(cfg, nil, "setup-failure", t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
			_, err := r.RunSubagentToolForTest(context.Background(), agentruntime.NestedRunConfig{
				Task: "inspect", SysPromptPath: filepath.Join(t.TempDir(), "missing.tmpl"),
				RunInBackground: background, Origin: chatstore.SubOriginParent,
				ProjectHex: "setup-failure", ToolCall: chatstore.ToolCall{ID: "setup-call", Name: "subagent"},
			})
			if !background && err == nil {
				t.Fatal("expected setup failure")
			}
			if background && err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				sessions, readErr := chatstore.ListSubSessions("setup-failure")
				if readErr != nil {
					t.Fatal(readErr)
				}
				active, readErr := chatstore.ReadActiveSubagents()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if len(sessions) == 1 && sessions[0].Status == chatstore.SubStatusPaused && len(active.Agents) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("setup failure left stale activity: sessions=%+v active=%+v", sessions, active)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
