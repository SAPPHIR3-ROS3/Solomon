package turnloop

import "testing"

func compileFailureForTest(hash, message string) map[string]any {
	return map[string]any{
		"ok":            false,
		"compile_error": message,
		"source_hash":   hash,
	}
}

func TestOrchestrateCompileRetryStopsDuplicate(t *testing.T) {
	state := orchestrateCompileRetryState{}
	first := compileFailureForTest("same", "invalid Go source")
	if terminal, _ := state.observe(first); terminal {
		t.Fatal("first compile failure should be retryable")
	}
	second := compileFailureForTest("same", "invalid Go source")
	terminal, reason := state.observe(second)
	if !terminal {
		t.Fatal("duplicate compile failure should stop retries")
	}
	if reason == "" || second["retryable"] != false || second["retry_exhausted"] != true {
		t.Fatalf("unexpected terminal result: %#v", second)
	}
}

func TestOrchestrateCompileRetryStopsAtAttemptLimit(t *testing.T) {
	state := orchestrateCompileRetryState{}
	for i := 1; i <= maxOrchestrateCompileAttempts; i++ {
		result := compileFailureForTest(string(rune('a'+i)), "compile error")
		terminal, _ := state.observe(result)
		if i < maxOrchestrateCompileAttempts && terminal {
			t.Fatalf("attempt %d should be retryable", i)
		}
		if i == maxOrchestrateCompileAttempts && !terminal {
			t.Fatal("attempt limit should stop retries")
		}
	}
}
