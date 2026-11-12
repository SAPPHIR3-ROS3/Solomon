package test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
)

func TestBuildFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name, source, kind string
	}{
		{"syntax", "package main\nfunc main(){ println(\"\\d\") }", "source"},
		{"type error", "package main\nimport \"strings\"\nfunc main(){println(strings.Index(\"abc\",\"b\",1))}", "source"},
		{"filesystem", "package main\nfunc main(){}", "filesystem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.kind == "filesystem" {
				home := t.TempDir()
				if err := os.WriteFile(filepath.Join(home, ".solomon"), []byte("blocked"), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("HOME", home)
			}
			_, err := compile.BuildWASM(compile.Options{Source: tc.source})
			if err == nil {
				t.Fatal("expected build failure")
			}
			info := compile.DescribeFailure(err)
			if info.Kind != tc.kind || (info.RetryAction == "fix_source") != (tc.kind == "source") {
				t.Fatalf("wrong classification: %#v: %v", info, err)
			}
			var typed *compile.Failure
			if !errors.As(err, &typed) {
				t.Fatalf("failure lost its type: %T", err)
			}
		})
	}
}

func TestFailureKindsPreserveCauses(t *testing.T) {
	for _, tc := range []struct {
		cause        error
		kind, action string
	}{
		{context.Canceled, "cancelled", "none"},
		{context.DeadlineExceeded, "timeout", "retry_build"},
		{&os.PathError{Op: "write", Path: "file", Err: os.ErrPermission}, "filesystem", "fix_environment"},
	} {
		info := compile.DescribeFailure(tc.cause)
		if info.Kind != tc.kind || info.RetryAction != tc.action || !errors.Is(info, tc.cause) {
			t.Fatalf("wrong failure: %#v", info)
		}
	}
}

func TestEnvironmentFailureDoesNotUseSourceRetryBudget(t *testing.T) {
	state := &turnloop.CompileRetryState{}
	for i := 0; i < 5; i++ {
		result := map[string]any{"ok": false, "error_type": "toolchain_error", "error_kind": "toolchain", "error": "Go not installed", "retryable": false}
		if terminal, _ := state.Observe(result); terminal {
			t.Fatal("environment failure consumed source retry budget")
		}
	}
	source := map[string]any{"compile_error": "invalid source", "source_hash": "new"}
	if terminal, _ := state.Observe(source); terminal {
		t.Fatal("first source failure should still be repairable")
	}
}

func TestEnvironmentFailureWireContract(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".solomon"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	raw, err := json.Marshal(map[string]string{"source": "package main\nfunc main(){}", "intent": "classify inaccessible build storage"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "orchestrate", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["error_type"] != "filesystem_error" || result["retry_action"] != "fix_environment" || result["retryable"] != false || result["compile_error"] != nil {
		t.Fatalf("environment treated as source error: %#v", result)
	}
}
