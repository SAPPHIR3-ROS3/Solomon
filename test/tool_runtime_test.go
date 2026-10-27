package test

import (
	"bytes"
	"context"
	"encoding/json"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeToolParamsAgentChat(t *testing.T) {
	agent, err := agenttools.NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(agent) != 12 {
		t.Fatalf("agent tools: %d", len(agent))
	}
	if agent[0].OfFunction.Function.Name != "docsRetrieval" {
		t.Fatalf("first agent tool: %s", agent[0].OfFunction.Function.Name)
	}
	names := map[string]bool{}
	for _, p := range agent {
		if p.OfFunction != nil {
			names[p.OfFunction.Function.Name] = true
		}
	}
	for _, want := range []string{"readChat", "searchSkill", "loadSkill", "installSkill", "searchTools", "orchestrate", "subagent", "listSubAgents", "switchMode", "settings", "customizeUI"} {
		if !names[want] {
			t.Fatalf("missing agent tool %s", want)
		}
	}
	chat, err := agenttools.NativeToolParams("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(chat) != 9 {
		t.Fatalf("chat tools: %d", len(chat))
	}
	if chat[0].OfFunction.Function.Name != "docsRetrieval" {
		t.Fatalf("first chat tool: %s", chat[0].OfFunction.Function.Name)
	}
}

func TestDocsRetrievalAllowedInChat(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{Cfg: config.EmptyRoot()}, "chat", tooling.Invocation{
		Name: "docsRetrieval",
		Args: json.RawMessage(`{"query":"checkpoint","intent":"read checkpoint documentation"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchModeUnchanged(t *testing.T) {
	env := &agenttools.Env{
		CurrentMode: func() string { return "agent" },
	}
	out, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "switchMode",
		Args: json.RawMessage(`{"mode":"agent","intent":"verify current mode"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["unchanged"] != true {
		t.Fatalf("got %v", m)
	}
}

func TestOrchestrateDeferredEditsShareCheckpointSeq(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(proj, "f.txt")
	if err := os.WriteFile(file, []byte("v0"), 0o600); err != nil {
		t.Fatal(err)
	}
	storeDir := filepath.Join(dir, "staging")
	store, err := staging.Load(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	const cpSeq = 7
	env := &agenttools.Env{
		ProjRoot:               proj,
		AllowDeferredTools:     true,
		CheckpointStageProjAbs: func(string) {},
		CheckpointBeforeProjAbs: func(path string) {
			_ = store.RecordBefore(path)
		},
		CheckpointRecordEdit: func(kind, path, renameTo string, content []byte) {
			_ = store.RecordOp(cpSeq, kind, path, renameTo, content)
		},
	}
	patch := func(body string) {
		args, _ := json.Marshal(map[string]any{
			"path": "f.txt", "oldString": body, "newString": body + "+", "intent": "test",
		})
		if _, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "editFile", Args: args}); err != nil {
			t.Fatal(err)
		}
	}
	patch("v0")
	patch("v0+")
	res, err := store.RestoreToCheckpoint(cpSeq-1, proj)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesWritten != 1 {
		t.Fatalf("written: %d", res.FilesWritten)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "v0" {
		t.Fatalf("got %q", b)
	}
}

func TestSwitchModeCountdownCompletes(t *testing.T) {
	t.Cleanup(agentruntime.ResetSwitchModeCountdownForTest)
	agentruntime.SetSwitchModeCountdownForTest(30*time.Millisecond, 8)
	rt := &agentruntime.Runtime{Mode: "agent", Out: &bytes.Buffer{}}
	cancelled, err := agentruntime.SwitchModeCountdownForTest(rt, context.Background(), "chat")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled {
		t.Fatal("expected complete")
	}
	if rt.Mode != "chat" {
		t.Fatalf("mode=%q", rt.Mode)
	}
}

func TestSwitchModeCountdownNonTTYDoesNotAccumulateProgressFrames(t *testing.T) {
	t.Cleanup(agentruntime.ResetSwitchModeCountdownForTest)
	agentruntime.SetSwitchModeCountdownForTest(5*time.Millisecond, 8)
	var out bytes.Buffer
	rt := &agentruntime.Runtime{Mode: "agent", Out: &out}

	cancelled, err := agentruntime.SwitchModeCountdownForTest(rt, context.Background(), "chat")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled {
		t.Fatal("expected complete")
	}
	if strings.Contains(out.String(), "\r") {
		t.Fatalf("non-TTY output contains carriage-return progress frames: %q", out.String())
	}
}

func TestSwitchModeCountdownCancelContext(t *testing.T) {
	t.Cleanup(agentruntime.ResetSwitchModeCountdownForTest)
	agentruntime.SetSwitchModeCountdownForTest(2*time.Second, 8)
	rt := &agentruntime.Runtime{Mode: "agent", Out: &bytes.Buffer{}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	cancelled, err := agentruntime.SwitchModeCountdownForTest(rt, ctx, "chat")
	if err == nil && !cancelled {
		t.Fatal("expected cancel")
	}
	if rt.Mode != "agent" {
		t.Fatalf("mode changed: %q", rt.Mode)
	}
}
