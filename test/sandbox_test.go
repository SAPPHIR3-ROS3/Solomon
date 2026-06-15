package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestSearchToolsQuery(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"edit file"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("got %T", out)
	}
	count, _ := m["count"].(int)
	if count == 0 {
		t.Fatal("expected hits")
	}
}

func TestSearchToolsGlobReadFilesQuery(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"Find Glob read files"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("got %T", out)
	}
	count, _ := m["count"].(int)
	if count < 2 {
		t.Fatalf("expected readFile and find hits, got count=%d", count)
	}
	raw, _ := json.Marshal(m["tools"])
	s := string(raw)
	if !strings.Contains(s, "readFile") || !strings.Contains(s, "find") {
		t.Fatalf("expected readFile and find in tools: %s", s)
	}
}

func TestSearchToolsEmptyQueryRejected(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":""}`),
	})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestAgentModeRejectsDirectReadFile(t *testing.T) {
	dir := t.TempDir()
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{ProjRoot: dir}, "agent", tooling.Invocation{
		Name: "readFile",
		Args: json.RawMessage(`{"path":"x.txt"}`),
	})
	if err == nil {
		t.Fatal("expected mode guard error")
	}
}

func TestAgentModeAllowsSearchSkill(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchSkill",
		Args: json.RawMessage(`{"query":"commit message"}`),
	})
	if err == nil {
		return
	}
	if !strings.Contains(err.Error(), "no matching skill") && !strings.Contains(err.Error(), "no skill reaches") && !strings.Contains(err.Error(), "no skills installed") {
		t.Fatalf("expected searchSkill to run in agent mode, got: %v", err)
	}
}

func TestAgentModeRejectsLoadSkillInAgentWithoutSkill(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "loadSkill",
		Args: json.RawMessage(`{"name":"definitely-missing-skill-xyz"}`),
	})
	if err == nil {
		t.Fatal("expected load error for missing skill")
	}
}

func TestDeferredHostAllowsReadFile(t *testing.T) {
	dir := t.TempDir()
	env := &agenttools.Env{ProjRoot: dir, AllowDeferredTools: true}
	_, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "readFile",
		Args: json.RawMessage(`{"path":"missing.txt"}`),
	})
	if err == nil {
		t.Fatal("expected read error for missing file")
	}
}

func TestOrchestrateCompileError(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "orchestrate",
		Args: json.RawMessage(`{"source":"this is not valid go"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if _, ok := m["compile_error"]; !ok {
		t.Fatalf("expected compile_error, got %v", m)
	}
}

func TestCompileMinimalWASM(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	if len(wasm) < 1000 {
		t.Fatalf("wasm too small: %d", len(wasm))
	}
}

func TestCompileSDKHelpers(t *testing.T) {
	src := `package main

import (
	sdk "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)

func main() {
	if err := sdk.WriteFile("a.txt", "line1\nline2\nline3", "seed"); err != nil {
		panic(err)
	}
	if err := sdk.ReplaceInFile("a.txt", "line2", "LINE2", "patch"); err != nil {
		panic(err)
	}
	r, err := sdk.ReadFileFromLineInfo("a.txt", 2)
	if err != nil {
		panic(err)
	}
	paths, err := sdk.GlobInLimit(".", "*.txt", 10)
	if err != nil {
		panic(err)
	}
	matches, err := sdk.GrepPathGlob("LINE2", "*.txt")
	if err != nil {
		panic(err)
	}
	lines, err := sdk.GrepLinesPathGlob("LINE2", "*.txt")
	if err != nil {
		panic(err)
	}
	er, err := sdk.WriteFileResult("a.txt", "line1\nLINE2\nline3", "seed")
	if err != nil || !er.OK {
		panic(err)
	}
	res, err := sdk.ShellResultWithTimeout("echo ok", "probe", 30)
	if err != nil {
		panic(err)
	}
	sdk.Printf("lines=%d paths=%d grep=%q grepLines=%d exit=%d action=%s\n", r.TotalLines, len(paths), matches, len(lines), res.Exit, er.Action)
}
`
	if _, err := compile.BuildWASM(compile.Options{Source: src}); err != nil {
		t.Fatal(err)
	}
}

func TestSearchToolsSDKHelpers(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"Glob"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	count, _ := m["count"].(int)
	if count == 0 {
		t.Fatal("expected sdk helper hits")
	}
}

type mockCaller struct {
	calls int
}

func (m *mockCaller) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	m.calls++
	return json.Marshal(map[string]any{"content": "hello"})
}

func TestRunnerExecutesMinimalWASM(t *testing.T) {
	src := `package main

import (
	"fmt"
	sdk "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)

func main() {
	c, err := sdk.ReadFile("foo.txt")
	if err != nil {
		panic(err)
	}
	fmt.Print(c)
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
	if mc.calls != 1 {
		t.Fatalf("tool calls: %d", mc.calls)
	}
	if out != "hello" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestSearchToolsExcludesSubagent(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"subagent"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("got %T", out)
	}
	count, _ := m["count"].(int)
	if count != 0 {
		t.Fatalf("subagent should not appear in deferred catalog, count=%d", count)
	}
}

func TestOrchestrateHostRejectsSubagent(t *testing.T) {
	env := &agenttools.Env{AllowDeferredTools: true}
	_, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "subagent",
		Args: json.RawMessage(`{"sysPromptPath":"agent.tmpl","task":"ok"}`),
	})
	if err == nil {
		t.Fatal("expected subagent rejected from orchestrate host")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentModeAllowsSubagentWhenRuntimeWired(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "sys.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &agenttools.Env{
		ProjRoot: dir,
		CurrentMode: func() string { return "agent" },
		SetMode:     func(string) {},
		RunSubagent: func(context.Context, agenttools.SubagentRequest) (agenttools.SubagentResponse, error) {
			return agenttools.SubagentResponse{Output: "OK", Status: "completed"}, nil
		},
	}
	_, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "subagent",
		Args: json.RawMessage(`{"sysPromptPath":"sys.txt","task":"ok"}`),
	})
	if err != nil {
		t.Fatalf("agent native subagent: %v", err)
	}
}
