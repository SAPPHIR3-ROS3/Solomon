package agentruntime

func (r *Runtime) setReplInputPrefill(s string) {
	r.replInputPrefillMu.Lock()
	r.replInputPrefill = s
	r.replInputPrefillMu.Unlock()
}

func (r *Runtime) takeReplInputPrefill() string {
	r.replInputPrefillMu.Lock()
	s := r.replInputPrefill
	r.replInputPrefill = ""
	r.replInputPrefillMu.Unlock()
	return s
}

func (r *Runtime) SetReplInputPrefillForTest(s string) {
	r.setReplInputPrefill(s)
}

func (r *Runtime) TakeReplInputPrefillForTest() string {
	return r.takeReplInputPrefill()
}
