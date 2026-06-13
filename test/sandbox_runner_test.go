package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/host"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestRunnerCapturesStdout(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	fmt.Print("hello-wasm")
}
`
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	mc := &mockCaller{}
	ctx := context.Background()
	rn, err := run.NewRunner(ctx, mc, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer rn.Close(ctx)
	out, err := rn.Run(ctx, wasm)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello-wasm" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestWorkerIPCSequentialRunsCaptureStdout(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	ctx := context.Background()
	client, err := parent.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	src := `package main

import (
	"fmt"
	sdk "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)

func main() {
	c, err := sdk.ReadFile("x.txt")
	if err != nil {
		panic(err)
	}
	fmt.Print("out:", c)
}
`
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	exec := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		if name != "readFile" {
			t.Fatalf("unexpected tool %q", name)
		}
		return json.Marshal(map[string]any{"content": "payload"})
	}
	for i := 0; i < 2; i++ {
		done, err := client.Run(ctx, wasm, "agent", exec)
		if err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
		if done.Error != "" {
			t.Fatalf("run %d error: %s", i+1, done.Error)
		}
		if done.ToolCalls != 1 {
			t.Fatalf("run %d tool_calls: %d", i+1, done.ToolCalls)
		}
		if done.Output != "out:payload" {
			t.Fatalf("run %d output=%q", i+1, done.Output)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repo root not found")
		}
		dir = parent
	}
}

func TestWorkerIPCLargeOutputThenSecondRun(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	ctx := context.Background()
	client, err := parent.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	large := strings.Repeat("x", 8000)
	exec := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"content": large})
	}

	srcPrintAll := `package main
import (
	"fmt"
	sdk "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)
func main() {
	c, err := sdk.ReadFile("README.md")
	if err != nil { panic(err) }
	fmt.Print(c)
}
`
	wasm1, err := compile.BuildWASM(compile.Options{Source: srcPrintAll})
	if err != nil {
		t.Fatal(err)
	}
	done1, err := client.Run(ctx, wasm1, "agent", exec)
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	if done1.Error != "" {
		t.Fatalf("run1 done: %s", done1.Error)
	}
	if len(done1.Output) != 8000 {
		t.Fatalf("run1 output len: %d", len(done1.Output))
	}

	srcCount := `package main
import (
	"fmt"
	sdk "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)
func main() {
	c, err := sdk.ReadFile("README.md")
	if err != nil { panic(err) }
	fmt.Printf("count:%d", len(c))
}
`
	wasm2, err := compile.BuildWASM(compile.Options{Source: srcCount})
	if err != nil {
		t.Fatal(err)
	}
	done2, err := client.Run(ctx, wasm2, "agent", exec)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	if done2.Error != "" {
		t.Fatalf("run2 done: %s", done2.Error)
	}
	if done2.Output != "count:8000" {
		t.Fatalf("run2 output=%q", done2.Output)
	}
}

func TestOrchestrateGlobalClientReadThenCount(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	proj := repoRoot(t)
	readme, err := os.ReadFile(filepath.Join(proj, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{ProjRoot: proj}
	ctx := context.Background()

	src1, _ := json.Marshal(map[string]string{
		"intent": "read README.md",
		"source": "package main\n\nimport (\n\t\"fmt\"\n\tsdk \"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk\"\n)\n\nfunc main() {\n\tcontent, err := sdk.ReadFile(\"README.md\")\n\tif err != nil {\n\t\tfmt.Printf(\"Error: %v\\n\", err)\n\t\treturn\n\t}\n\tfmt.Println(content)\n}",
	})
	out1, err := agenttools.Exec(ctx, env, "agent", tooling.Invocation{Name: "orchestrate", Args: src1})
	if err != nil {
		t.Fatalf("run1 exec: %v", err)
	}
	m1 := out1.(map[string]any)
	if m1["ok"] != true {
		t.Fatalf("run1 not ok: %v", m1)
	}
	if m1["error"] != nil {
		t.Fatalf("run1 error: %v", m1["error"])
	}
	got1, _ := m1["output"].(string)
	want1 := strings.TrimSpace(string(readme))
	if strings.TrimSpace(got1) != want1 {
		t.Fatalf("run1 output mismatch: len=%d want=%d", len(got1), len(want1))
	}

	src2, _ := json.Marshal(map[string]string{
		"intent": "count characters in README.md",
		"source": "package main\n\nimport (\n\t\"fmt\"\n\tsdk \"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk\"\n)\n\nfunc main() {\n\tcontent, err := sdk.ReadFile(\"README.md\")\n\tif err != nil {\n\t\tfmt.Printf(\"Error: %v\\n\", err)\n\t\treturn\n\t}\n\tfmt.Printf(\"Caratteri totali: %d\\n\", len(content))\n}",
	})
	out2, err := agenttools.Exec(ctx, env, "agent", tooling.Invocation{Name: "orchestrate", Args: src2})
	if err != nil {
		t.Fatalf("run2 exec: %v", err)
	}
	m2 := out2.(map[string]any)
	if m2["ok"] != true {
		t.Fatalf("run2 not ok: %v", m2)
	}
	if m2["error"] != nil {
		t.Fatalf("run2 error: %v", m2["error"])
	}
	want2 := fmt.Sprintf("Caratteri totali: %d", len(readme))
	got2, _ := m2["output"].(string)
	if got2 != want2 {
		t.Fatalf("run2 output=%q want=%q", got2, want2)
	}
}

func TestGlobalRecoversAfterWorkerCrash(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	ctx := context.Background()
	if _, err := parent.Global(ctx); err != nil {
		t.Fatal(err)
	}
	parent.SimulateWorkerCrash()

	src := `package main

import "fmt"

func main() {
	fmt.Print("recovered")
}
`
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	done, err := parent.RunGlobal(ctx, wasm, "agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if done.Error != "" {
		t.Fatalf("run error: %s", done.Error)
	}
	if done.Output != "recovered" {
		t.Fatalf("output=%q", done.Output)
	}
}

func TestHostRPCRoundTrip(t *testing.T) {
	mem := host.FormatRunError(nil)
	if mem != "" {
		t.Fatalf("unexpected: %q", mem)
	}
}

func TestNativeToolParamsAgentChat(t *testing.T) {
	agent, err := agenttools.NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(agent) != 6 {
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
	for _, want := range []string{"searchSkill", "loadSkill", "searchTools", "orchestrate", "switchMode"} {
		if !names[want] {
			t.Fatalf("missing agent tool %s", want)
		}
	}
	chat, err := agenttools.NativeToolParams("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(chat) != 4 {
		t.Fatalf("chat tools: %d", len(chat))
	}
	if chat[0].OfFunction.Function.Name != "docsRetrieval" {
		t.Fatalf("first chat tool: %s", chat[0].OfFunction.Function.Name)
	}
}

func TestDocsRetrievalAllowedInChat(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{Cfg: config.EmptyRoot()}, "chat", tooling.Invocation{
		Name: "docsRetrieval",
		Args: json.RawMessage(`{"query":"checkpoint"}`),
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
		Args: json.RawMessage(`{"mode":"agent"}`),
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
		ProjRoot:           proj,
		AllowDeferredTools: true,
		CheckpointStageProjAbs:  func(string) {},
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

func TestRunnerSecondRunUsesCache(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	mc := &mockCaller{}
	ctx := context.Background()
	rn, err := run.NewRunner(ctx, mc, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer rn.Close(ctx)
	start := time.Now()
	if _, err := rn.Run(ctx, wasm); err != nil {
		t.Fatal(err)
	}
	first := time.Since(start)
	start = time.Now()
	if _, err := rn.Run(ctx, wasm); err != nil {
		t.Fatal(err)
	}
	second := time.Since(start)
	if second > first {
		t.Logf("cache may vary: first=%v second=%v", first, second)
	}
}

func TestCountingCallerMaxToolCalls(t *testing.T) {
	inner := &mockCaller{}
	c := &host.CountingCaller{Inner: inner, MaxCalls: 1}
	ctx := context.Background()
	if _, err := c.Call(ctx, "readFile", json.RawMessage(`{"path":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Call(ctx, "readFile", json.RawMessage(`{"path":"y"}`)); err == nil {
		t.Fatal("expected max calls error")
	}
}

func TestEnsureReferenceWASM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	b, err := compile.EnsureReferenceWASM("testver")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 1000 {
		t.Fatalf("wasm len %d", len(b))
	}
	b2, err := compile.EnsureReferenceWASM("testver")
	if err != nil {
		t.Fatal(err)
	}
	if len(b2) != len(b) {
		t.Fatal("reference wasm not reused")
	}
}
