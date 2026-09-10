package turnloop

import "fmt"

const maxOrchestrateCompileAttempts = 3

type orchestrateCompileRetryState struct {
	attempts        int
	lastFingerprint string
	exhausted       bool
	terminalMessage string
}

func (s *orchestrateCompileRetryState) reset() {
	s.attempts = 0
	s.lastFingerprint = ""
	s.exhausted = false
	s.terminalMessage = ""
}

func (s *orchestrateCompileRetryState) observe(result any) (bool, string) {
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
	m["max_attempts"] = maxOrchestrateCompileAttempts
	m["retryable"] = true

	if duplicate {
		s.terminalMessage = "the same orchestrate source produced the same compile error twice"
	} else if s.attempts >= maxOrchestrateCompileAttempts {
		s.terminalMessage = fmt.Sprintf("orchestrate compile retry limit reached (%d attempts)", maxOrchestrateCompileAttempts)
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
