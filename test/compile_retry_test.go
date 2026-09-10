package test

import (
	"testing"

	turnloop "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
)

func compileFailureForTest(hash, message string) map[string]any {
	return map[string]any{
		"ok":            false,
		"compile_error": message,
		"source_hash":   hash,
	}
}

func TestOrchestrateCompileRetryStopsDuplicate(t *testing.T) {
	state := turnloop.CompileRetryState{}
	first := compileFailureForTest("same", "invalid Go source")
	if terminal, _ := state.Observe(first); terminal {
		t.Fatal("first compile failure should be retryable")
	}
	second := compileFailureForTest("same", "invalid Go source")
	terminal, reason := state.Observe(second)
	if !terminal {
		t.Fatal("duplicate compile failure should stop retries")
	}
	if reason == "" || second["retryable"] != false || second["retry_exhausted"] != true {
		t.Fatalf("unexpected terminal result: %#v", second)
	}
}

func TestOrchestrateCompileRetryStopsAtAttemptLimit(t *testing.T) {
	state := turnloop.CompileRetryState{}
	for i := 1; i <= turnloop.MaxOrchestrateCompileAttempts; i++ {
		result := compileFailureForTest(string(rune('a'+i)), "compile error")
		terminal, _ := state.Observe(result)
		if i < turnloop.MaxOrchestrateCompileAttempts && terminal {
			t.Fatalf("attempt %d should be retryable", i)
		}
		if i == turnloop.MaxOrchestrateCompileAttempts && !terminal {
			t.Fatal("attempt limit should stop retries")
		}
	}
}
