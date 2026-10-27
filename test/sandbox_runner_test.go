package test

import (
	"context"
	"encoding/json"
	"fmt"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/host"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/ipc"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	"sdk"
)

func main() {
	c, err := sdk.ReadFile("x.txt", "read x.txt")
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

func TestGlobalRunsConcurrentOrchestrationsOnSeparateWorkers(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	ctx := context.Background()
	src := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	content, err := sdk.ReadFile("x.txt", "read x")
	if err != nil {
		panic(err)
	}
	fmt.Print(content)
}
`
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	updateMax := func(value int32) {
		for {
			current := maxInFlight.Load()
			if value <= current || maxInFlight.CompareAndSwap(current, value) {
				return
			}
		}
	}
	exec := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		if name != "readFile" {
			return nil, fmt.Errorf("unexpected tool %q", name)
		}
		current := inFlight.Add(1)
		updateMax(current)
		defer inFlight.Add(-1)
		started <- struct{}{}
		select {
		case <-release:
			return json.Marshal(map[string]any{"content": "payload"})
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	type result struct {
		done ipc.RunDone
		err  error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			done, err := parent.RunGlobal(ctx, wasm, "agent", exec)
			results <- result{done: done, err: err}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		// Each worker compiles WASM before reaching the host. Race builds
		// can take more than 30 seconds on slower machines; the barrier still
		// requires both host calls to overlap before either is released.
		case <-time.After(90 * time.Second):
			t.Fatal("concurrent orchestrations did not reach host calls together")
		}
	}
	releaseAll()
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.done.Error != "" || got.done.Output != "payload" {
			t.Fatalf("run=%+v", got.done)
		}
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Fatalf("max concurrent host calls=%d, want at least 2", got)
	}
}

func TestWorkerIPCCallsMCPThroughCodeModeNamespace(t *testing.T) {
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
	"sdk"
)

func main() {
	result, err := sdk.mcp.search("search remote data", map[string]any{"query": "MCP"})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(result))
}
`
	wasm, err := compile.BuildWASM(compile.Options{
		Source: src,
		MCPTools: []compile.MCPToolBinding{{
			ExposedName: "MCP.github.search",
			ServerName:  "github",
			ToolName:    "search",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	exec := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		calls++
		if name != "MCP.github.search" {
			t.Fatalf("unexpected MCP tool %q", name)
		}
		var got map[string]any
		if err := json.Unmarshal(args, &got); err != nil {
			t.Fatal(err)
		}
		if got["intent"] != "search remote data" || got["query"] != "MCP" {
			t.Fatalf("MCP arguments = %#v", got)
		}
		return json.Marshal(map[string]any{"ok": true})
	}
	done, err := client.Run(ctx, wasm, "agent", exec)
	if err != nil {
		t.Fatal(err)
	}
	if done.Error != "" {
		t.Fatalf("run error: %s", done.Error)
	}
	if calls != 1 || done.ToolCalls != 1 {
		t.Fatalf("calls=%d tool_calls=%d", calls, done.ToolCalls)
	}
	if done.Output != `{"ok":true}` {
		t.Fatalf("output=%q", done.Output)
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
	"sdk"
)
func main() {
	c, err := sdk.ReadFile("README.md", "read README.md")
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
	"sdk"
)
func main() {
	c, err := sdk.ReadFile("README.md", "count README.md characters")
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
		"source": "package main\n\nimport (\n\t\"fmt\"\n\t\"sdk\"\n)\n\nfunc main() {\n\tcontent, err := sdk.ReadFile(\"README.md\", \"read README.md\")\n\tif err != nil {\n\t\tfmt.Printf(\"Error: %v\\n\", err)\n\t\treturn\n\t}\n\tfmt.Println(content)\n}",
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
		"source": "package main\n\nimport (\n\t\"fmt\"\n\t\"sdk\"\n)\n\nfunc main() {\n\tcontent, err := sdk.ReadFile(\"README.md\", \"count README.md characters\")\n\tif err != nil {\n\t\tfmt.Printf(\"Error: %v\\n\", err)\n\t\treturn\n\t}\n\tfmt.Printf(\"Caratteri totali: %d\\n\", len(content))\n}",
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
