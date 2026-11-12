package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mediaartifact"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/shellpolicy"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func signatureOrchestrate(source, intent string, inputs map[string]string) {}

type orchestrateArgs struct {
	Source string            `json:"source"`
	Inputs map[string]string `json:"inputs,omitempty"`
	Intent string            `json:"intent"`
}

func orchestrateOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("orchestrate", shellpolicy.Guidance+" "+compile.GoGuidance+" "+compile.InputGuidance+" "+outcomeGuidance+" Run a Go orchestration script (package main) that calls deferred Solomon tools via the sandbox SDK. Use searchTools for exact SDK signatures before writing the script. SDK helpers include file reads/edits, Find/Glob/Grep, ListDir/Tree, Shell, web/docs/research, plan helpers, and connected MCP tools through sdk.mcp.<tool>(intent, args). Scripts run in WASM: use sdk.Shell for host commands (not os/exec). sdk.Shell returns (string, error) — assign output to a variable and fmt.Println it; bare sdk.Shell calls do not appear in the tool result. Read/transform/write files via SDK — do not paste large bodies with backticks into Go raw strings. Only fmt.Print/Println/Printf in main() is captured in the tool result output field. Compile failures return error_type=compile_error with source_hash, attempt, max_attempts, and retryable; do not repeat an identical source after a compile failure. In Go, := requires at least one new variable in the current scope; use = to reassign existing variables, or separate repeated operations into blocks.", map[string]any{
		"inputs": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "Optional named plain-text inputs, JSON-escaped once. Put regexes, replacements, commands, and multiline payloads here instead of embedding them in Go source. Access with sdk.Input(name) (string, error)."},
		"source": map[string]any{"type": "string", "description": "Complete Go source: package main, import sandbox SDK via import \"sdk\", func main(); use the exact signatures returned by searchTools"},
		"intent": map[string]any{"type": "string", "description": "Brief phrase describing what this script does"},
	}, []string{"source", "intent"})
}

func appendOrchestrateDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureOrchestrate)
	if err != nil {
		return err
	}
	b.addBlock("orchestrate", shellpolicy.Guidance+" "+compile.GoGuidance+" "+compile.InputGuidance+" "+outcomeGuidance+" Run multi-tool Go scripts compiled to WASM. Import sandbox SDK via import \"sdk\" only and use the exact signatures from searchTools. Helpers include ReadFile, ReadFileLines, ReplaceInFile, WriteFile, DeleteFile, RenameFile, Find/Glob/Grep, ListDir/Tree, Shell (not os/exec), WebSearch, FetchWeb, DocsRetrieval, plan and research helpers, plus connected MCP tools through sdk.mcp.<tool>(intent, args). Do not embed markdown backticks in Go raw strings — ReadFile/transform/WriteFile instead. sdk.Shell returns (string, error): assign to a variable and fmt.Println it — bare calls are invisible in the tool result. Compile failures include source_hash and retry metadata; fix the source and do not repeat an identical failed script. Use = when reassigning existing variables; := requires a new variable in the current scope. Put repeated operations in separate blocks when reusing local names. Only fmt.Print/Println/Printf in main() is captured in the tool result output field.", sig)
	return nil
}

func execOrchestrate(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a orchestrateArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if env == nil {
		return nil, fmt.Errorf("orchestrate: runtime environment unavailable")
	}
	cacheDir, cacheErr := compile.CacheDir()
	if cacheErr != nil {
		return orchestrateCompileErrorResult(a.Source, cacheErr), nil
	}
	var mcpTools []compile.MCPToolBinding
	if env.MCP != nil {
		for _, tool := range env.MCP.Catalog() {
			mcpTools = append(mcpTools, compile.MCPToolBinding{
				ExposedName: tool.Name,
				ServerName:  tool.Server,
				ToolName:    tool.Tool,
			})
		}
	}
	wasm, err := compile.BuildWASM(compile.Options{Source: a.Source, Inputs: a.Inputs, CacheDir: cacheDir, MCPTools: mcpTools})
	if err != nil {
		failure := orchestrateCompileErrorResult(a.Source, err)
		logging.Log(logging.WARNING_LOG_LEVEL, "orchestrate build failed", logging.LogOptions{Params: map[string]any{"err": err.Error(), "error_kind": failure["error_kind"], "phase": failure["phase"], "retry_action": failure["retry_action"], "retryable": failure["retryable"]}})
		return failure, nil
	}
	outcomes := &orchestrateOutcomes{}
	var attachments []images.Attachment
	var attachmentsMu sync.Mutex
	done, err := parent.RunGlobal(ctx, wasm, deferredExecMode(env), func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		result, callErr := orchestrateHostCall(ctx, env, name, args, func(imgs []images.Attachment) {
			attachmentsMu.Lock()
			defer attachmentsMu.Unlock()
			attachments = append(attachments, imgs...)
		})
		outcomes.record(name, result, callErr)
		return result, callErr
	})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "orchestrate sandbox run failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		failure := orchestrateRunFailure(err)
		outcomes.apply(failure, false)
		return failure, nil
	}
	out := map[string]any{
		"ok":          done.Error == "",
		"tool_calls":  done.ToolCalls,
		"duration_ms": done.DurationMs,
	}
	outcomes.apply(out, done.Error == "")
	if strings.TrimSpace(done.Output) != "" {
		out["output"] = done.Output
	}
	if done.Error != "" {
		out["error"] = done.Error
		out["error_kind"] = "runtime"
		out["error_type"] = "runtime_error"
		out["phase"] = "execution"
		out["retryable"] = false
		out["retry_action"] = "inspect_execution"
	}
	if len(attachments) > 0 {
		out[images.ToolAttachmentsKey] = attachments
		vision, previews := 0, 0
		for _, a := range attachments {
			if a.PresentationOnly {
				previews++
			} else {
				vision++
			}
		}
		if vision > 0 {
			out["images_attached"] = vision
		}
		if previews > 0 {
			out["media_previews"] = previews
		}
	}
	return out, nil
}

func orchestrateCompileErrorResult(source string, err error) map[string]any {
	hash := sha256.Sum256([]byte(source))
	failure := compile.DescribeFailure(err)
	result := map[string]any{
		"ok": false, "status": "error", "phase": failure.Phase,
		"error_kind": failure.Kind, "error_type": failure.Kind + "_error",
		"error": err.Error(), "retryable": failure.Retryable,
		"retry_action": failure.RetryAction, "source_hash": hex.EncodeToString(hash[:]),
	}
	if failure.Kind == "source" {
		result["stage"] = failure.Phase
		result["phase"] = "compile"
		result["error_type"] = "compile_error"
		result["compile_error"] = err.Error()
		diagnostics := compile.SourceDiagnostics(source)
		if len(diagnostics) == 0 {
			diagnostics = compile.CompilerDiagnostics(err.Error())
		}
		if len(diagnostics) > 0 {
			result["diagnostics"] = diagnostics
			result["input_guidance"] = compile.InputGuidance
		}
	}
	return result
}

func orchestrateHostCall(ctx context.Context, env *Env, name string, args json.RawMessage, collect func([]images.Attachment)) (json.RawMessage, error) {
	hostEnv := *env
	hostEnv.AllowDeferredTools = true
	hostEnv.ParentToolName = "orchestrate"
	result, err := Exec(ctx, &hostEnv, "agent", tooling.Invocation{Name: name, Args: args})
	if err != nil {
		return nil, err
	}
	result, attachments := images.TakeToolAttachments(result)
	attachments = mediaartifact.Capture(result, env.ProjRoot, attachments)
	if collect != nil && len(attachments) > 0 {
		collect(attachments)
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
