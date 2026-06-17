package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func signatureOrchestrate(source, intent string) {}

type orchestrateArgs struct {
	Source string `json:"source"`
	Intent string `json:"intent"`
}

func orchestrateOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("orchestrate", "Run a Go orchestration script (package main) that calls deferred Solomon tools via the sandbox SDK. Use searchTools for SDK signatures and deferred tool catalog. Scripts run in WASM: use sdk.Shell for host commands (not os/exec). sdk.Shell returns (string, error) — assign output to a variable and fmt.Println it; bare sdk.Shell calls do not appear in the tool result. Read/transform/write files via SDK — do not paste large bodies with backticks into Go raw strings. Only fmt.Print/Println/Printf in main() is captured in the tool result output field.", map[string]any{
		"source": map[string]any{"type": "string", "description": "Complete Go source: package main, import sandbox SDK via import \"sdk\", func main()"},
		"intent": map[string]any{"type": "string", "description": "Brief phrase describing what this script does"},
	}, []string{"source", "intent"})
}

func appendOrchestrateDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureOrchestrate)
	if err != nil {
		return err
	}
	b.addBlock("orchestrate", "Run multi-tool Go scripts compiled to WASM. Import sandbox SDK via import \"sdk\" only. Helpers: ReadFile, ReplaceInFile(path,old,new,intent), WriteFile(path,content,intent), Glob/Grep, Shell (not os/exec), WebSearch, FetchWeb, DocsRetrieval. Do not embed markdown backticks in Go raw strings — ReadFile/transform/WriteFile instead. sdk.Shell returns (string, error): assign to a variable and fmt.Println it — bare calls are invisible in the tool result. Only fmt.Print/Println/Printf in main() is captured in the tool result output field.", sig)
	return nil
}

func execOrchestrate(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a orchestrateArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	cacheDir, _ := compile.CacheDir()
	wasm, err := compile.BuildWASM(compile.Options{Source: a.Source, CacheDir: cacheDir})
	if err != nil {
		return map[string]any{"ok": false, "compile_error": err.Error()}, nil
	}
	parent.Warm(ctx, "")
	done, err := parent.RunGlobal(ctx, wasm, deferredExecMode(env), func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		return orchestrateHostCall(ctx, env, name, args)
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"ok":          done.Error == "",
		"tool_calls":  done.ToolCalls,
		"duration_ms": done.DurationMs,
	}
	if strings.TrimSpace(done.Output) != "" {
		out["output"] = done.Output
	}
	if done.Error != "" {
		out["error"] = done.Error
	}
	return out, nil
}

func orchestrateHostCall(ctx context.Context, env *Env, name string, args json.RawMessage) (json.RawMessage, error) {
	hostEnv := *env
	hostEnv.AllowDeferredTools = true
	result, err := Exec(ctx, &hostEnv, "agent", tooling.Invocation{Name: name, Args: args})
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func deferredExecMode(env *Env) string {
	if env.CurrentMode != nil {
		if m := env.CurrentMode(); m != "" {
			return m
		}
	}
	return "agent"
}
