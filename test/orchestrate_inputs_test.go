package test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestOrchestrateInputsPreservePayloadsAndRunIsolation(t *testing.T) {
	inputs := map[string]string{
		"pattern":     `\b(foo|bar)\s+\d+\.\w+`,
		"replacement": "$1\\path\"quoted\"\nnext\tcolumn",
		"windows":     `C:\Users\example\file.txt`,
		"markdown":    "```go\nfmt.Println(`hello`)\n```",
		"unicode":     "città 日本語 😀\r\n\x00end",
		"injection":   "\"); }\nfunc init() { panic(\"injected\") }\n//",
		"empty":       "",
		"key\"\\\n`":  "unusual key",
	}
	source := `package main
import (
 "encoding/json"
 "fmt"
 "sdk"
)
func main() {
 keys, err := sdk.Input("keys")
 if err != nil { fmt.Print("missing keys"); return }
 var names []string
 if err := json.Unmarshal([]byte(keys), &names); err != nil { panic(err) }
 values := map[string]string{}
 for _, name := range names {
  value, err := sdk.Input(name)
  if err != nil { panic(err) }
  values[name] = value
 }
 encoded, err := json.Marshal(values)
 if err != nil { panic(err) }
 fmt.Print(string(encoded))
}`
	var keys []string
	for key := range inputs {
		keys = append(keys, key)
	}
	encodedKeys, err := json.Marshal(keys)
	if err != nil {
		t.Fatal(err)
	}
	inputs["keys"] = string(encodedKeys)
	wasm, err := compile.BuildWASM(compile.Options{Source: source, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	caller := &mockCaller{}
	runner, err := run.NewRunner(ctx, caller, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close(ctx)
	output, err := runner.Run(ctx, wasm)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("invalid output %q: %v", output, err)
	}
	delete(inputs, "keys")
	if len(got) != len(inputs) {
		t.Fatalf("got %d inputs, want %d", len(got), len(inputs))
	}
	for key, value := range inputs {
		if actual, exists := got[key]; !exists || actual != value {
			t.Fatalf("input %q changed: got %q, want %q", key, actual, value)
		}
	}
	if caller.calls != 0 {
		t.Fatalf("input reads unexpectedly invoked %d host tools", caller.calls)
	}
	withoutInputs, err := compile.BuildWASM(compile.Options{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	output, err = runner.Run(ctx, withoutInputs)
	if err != nil || output != "missing keys" {
		t.Fatalf("inputs leaked into next run: output=%q err=%v", output, err)
	}
}

func TestOrchestrateEscapeDiagnostics(t *testing.T) {
	source := "package main\nimport \"fmt\"\nfunc main() { fmt.Print(\"\\d+\") }\n"
	diagnostics := compile.SourceDiagnostics(source)
	if len(diagnostics) == 0 {
		t.Fatal("expected invalid escape diagnostics")
	}
	first := diagnostics[0]
	if first.Line != 3 || first.Column <= 0 || first.Message != "unknown escape sequence" || !strings.Contains(first.Hint, "sdk.Input") {
		t.Fatalf("unexpected diagnostic: %#v", first)
	}
	args, err := json.Marshal(map[string]any{"source": source, "intent": "inspect escape recovery", "inputs": map[string]string{"pattern": `\d+`}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "orchestrate", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["error_type"] != "compile_error" || m["diagnostics"] == nil || m["input_guidance"] == nil {
		t.Fatalf("missing structured escape guidance: %#v", m)
	}
}

func TestOrchestrateValidEscapeSemantics(t *testing.T) {
	source := `package main
import "fmt"
func main() { fmt.Print("begin\n\t\\\"end") }
`
	if diagnostics := compile.SourceDiagnostics(source); len(diagnostics) != 0 {
		t.Fatalf("valid escapes were rejected: %#v", diagnostics)
	}
	wasm, err := compile.BuildWASM(compile.Options{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	runner, err := run.NewRunner(ctx, &mockCaller{}, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close(ctx)
	output, err := runner.Run(ctx, wasm)
	if err != nil || output != "begin\n\t\\\"end" {
		t.Fatalf("valid escape semantics changed: output=%q err=%v", output, err)
	}
}

func TestOrchestrateInputsToolRoundTrip(t *testing.T) {
	payload := "\\d+\n`quoted` \"text\"\x00"
	source := `package main
import ("fmt"; "sdk")
func main() {
 value, err := sdk.Input("payload")
 if err != nil { panic(err) }
 fmt.Print(value)
}`
	args, err := json.Marshal(map[string]any{
		"source": source, "inputs": map[string]string{"payload": payload},
		"intent": "verify literal input delivery",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "orchestrate", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["ok"] != true || m["output"] != payload || m["tool_calls"] != 0 {
		t.Fatalf("unexpected input round trip: %#v", m)
	}
}
