package turnloop

import "fmt"

// MaxOrchestrateCompileAttempts is the maximum number of compile failures tolerated for one orchestrate request.
const MaxOrchestrateCompileAttempts = 3

// CompileRetryState tracks compile failures for one orchestrate request.
type CompileRetryState struct {
	attempts        int
	lastFingerprint string
	exhausted       bool
	terminalMessage string
}

func (s *CompileRetryState) Reset() {
	s.attempts = 0
	s.lastFingerprint = ""
	s.exhausted = false
	s.terminalMessage = ""
}

func (s *CompileRetryState) Observe(result any) (bool, string) {
	m, ok := result.(map[string]any)
	if !ok {
		return false, ""
	}
	compileError, ok := m["compile_error"].(string)
	if !ok || compileError == "" {
		return false, ""
	}
	if s.exhausted {
		return true, s.terminalMessage
	}

	sourceHash, _ := m["source_hash"].(string)
	fingerprint := sourceHash + "\x00" + compileError
	if sourceHash == "" {
		fingerprint = compileError
	}
	duplicate := fingerprint == s.lastFingerprint
	s.lastFingerprint = fingerprint
	s.attempts++
	m["attempt"] = s.attempts
	m["max_attempts"] = MaxOrchestrateCompileAttempts
	m["retryable"] = true

	if duplicate {
		s.terminalMessage = "the same orchestrate source produced the same compile error twice"
	} else if s.attempts >= MaxOrchestrateCompileAttempts {
		s.terminalMessage = fmt.Sprintf("orchestrate compile retry limit reached (%d attempts)", MaxOrchestrateCompileAttempts)
	} else {
		return false, ""
	}

	s.exhausted = true
	m["error"] = s.terminalMessage
	m["retryable"] = false
	m["retry_exhausted"] = true
	return true, s.terminalMessage
}

func hasOrchestrateCompileError(result any) bool {
	m, ok := result.(map[string]any)
	if !ok {
		return false
	}
	compileError, ok := m["compile_error"].(string)
	return ok && compileError != ""
}
