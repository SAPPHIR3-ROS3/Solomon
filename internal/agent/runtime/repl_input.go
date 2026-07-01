package agentruntime

import (
	"os"
	"strings"
)

func (r *Runtime) setReplInputPrefill(s string) {
	r.replInputPrefillMu.Lock()
	r.replInputPrefill = s
	r.replInputPrefillMu.Unlock()
}

func (r *Runtime) takeReplInputPrefill() string {
	r.replInputPrefillMu.Lock()
	defer r.replInputPrefillMu.Unlock()
	s := r.replInputPrefill
	r.replInputPrefill = ""
	if s == "" && !r.replInputPrefillEnvUsed {
		if env := strings.TrimSpace(os.Getenv("SOLOMON_REPL_PREFILL")); env != "" {
			s = env
		}
	}
	r.replInputPrefillEnvUsed = true
	return s
}

func (r *Runtime) SetReplInputPrefillForTest(s string) {
	r.setReplInputPrefill(s)
}

func (r *Runtime) TakeReplInputPrefillForTest() string {
	return r.takeReplInputPrefill()
}
