package test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestGoCompilerDiagnosticCategories(t *testing.T) {
	cases := []struct{ message, category, hint string }{
		{"no new variables on left side of :=", "declaration", "Use ="},
		{"assignment mismatch: 2 variables but strings.Index returns 1 value", "return_values", "one int"},
		{`"fmt" imported and not used`, "import", "Remove"},
		{"undefined: strings", "undefined_symbol", "explicit import"},
		{"undefined: sdk.ReadText", "sdk_signature", "searchTools"},
		{"not enough arguments in call to sdk.ReadFile", "sdk_signature", "searchTools"},
		{"assignment mismatch: 1 variable but sdk.Shell returns 2 values", "sdk_signature", "return values"},
		{"declared and not used: value", "unused_variable", "remove"},
		{"text.Split undefined (type string has no field or method Split)", "member", "strings"},
	}
	for _, tc := range cases {
		t.Run(tc.category+tc.message, func(t *testing.T) {
			got := compile.CompilerDiagnostics("compile: # generated\n./main.go:7:3: " + tc.message + "\n")
			if len(got) != 1 || got[0].Line != 7 || got[0].Column != 3 || got[0].Category != tc.category || !strings.Contains(got[0].Hint, tc.hint) {
				t.Fatalf("unexpected diagnostics: %#v", got)
			}
		})
	}
	if got := compile.CompilerDiagnostics("toolchain unavailable\nother.go:1:2: undefined: x"); len(got) != 0 {
		t.Fatalf("unrelated failure parsed: %#v", got)
	}
	output := strings.Repeat("main.go:1: undefined: x\n", 20)
	if got := compile.CompilerDiagnostics(output); len(got) != 8 {
		t.Fatalf("expected bounded diagnostics, got %d", len(got))
	}
}

func TestOrchestrateGoTypeErrorsHaveRecoveryDiagnostics(t *testing.T) {
	source := `package main
import "strings"
func main() { index, found := strings.Index("abc", "b"); _, _ = index, found }
`
	raw, err := json.Marshal(map[string]any{"source": source, "intent": "verify Go return-value diagnostics"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "orchestrate", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	got := result.(map[string]any)
	diagnostics, ok := got["diagnostics"].([]compile.SourceDiagnostic)
	if got["error_type"] != "compile_error" || !ok || len(diagnostics) == 0 || diagnostics[0].Category != "return_values" || diagnostics[0].Line != 3 {
		t.Fatalf("unexpected failure: %#v", got)
	}
}
