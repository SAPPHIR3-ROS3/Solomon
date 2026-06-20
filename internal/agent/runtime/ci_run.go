package agentruntime

import (
	"context"
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func (r *Runtime) machineMode() bool {
	return r.EventSink != nil
}

func (r *Runtime) ciReportMeta() cievents.ReportMeta {
	prov := ""
	if r.Prov != nil {
		prov = r.Prov.Name
	}
	return cievents.ReportMeta{
		Prompt:    r.ciPrompt,
		Model:     r.Model,
		Provider:  prov,
		ProjHex:   r.ProjHex,
		Ephemeral: r.EphemeralSession,
	}
}

func (r *Runtime) ciEmit(ev cievents.Event) {
	if r.EventSink != nil {
		r.EventSink.Emit(ev)
	}
}

func toolCallsCI(tcs []chatstore.ToolCall) []map[string]any {
	if len(tcs) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(tcs))
	for _, tc := range tcs {
		var args any
		if tc.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Arguments), &args)
		}
		if args == nil {
			args = map[string]any{}
		}
		out = append(out, map[string]any{"id": tc.ID, "name": tc.Name, "arguments": args})
	}
	return out
}

func resultMapHasError(res any) bool {
	m, ok := res.(map[string]any)
	if !ok {
		return false
	}
	if _, ok := m["error"]; ok {
		return true
	}
	return false
}

func (r *Runtime) noteCIToolResult(res any) {
	if resultMapHasError(res) {
		r.ciToolErr = true
	}
}

func (r *Runtime) wrapLLMErr(err error) error {
	if err == nil {
		return nil
	}
	code, reason := cievents.ClassifyExit(err)
	if code == cievents.ExitTimeout {
		return cievents.TimeoutError(err)
	}
	return cievents.NewRunError(cievents.ExitLLM, reason, err)
}

func (r *Runtime) runPromptOnceCI(ctx context.Context, line string) (err error) {
	r.ciPrompt = line
	r.ciTurn = 0
	r.ciToolErr = false
	r.ciFinalContent = ""
	prov := ""
	if r.Prov != nil {
		prov = r.Prov.Name
	}
	r.ciEmit(cievents.RunStart(line, r.Model, prov, r.ProjHex, r.EphemeralSession))
	defer func() {
		if err != nil {
			code, msg := cievents.ClassifyExit(err)
			logging.Log(logging.ERROR_LOG_LEVEL, "CI run failed", logging.LogOptions{Params: map[string]any{"code": code, "reason": msg, "err": err.Error()}})
			r.ciEmit(cievents.ErrorEvent(code, msg))
		}
		exitCode, exitReason := cievents.ClassifyExit(err)
		if r.FailOnToolError && r.ciToolErr && err == nil {
			exitCode = cievents.ExitTool
			exitReason = "tool_error"
			err = cievents.ToolPolicyError()
		}
		r.ciEmit(cievents.RunEnd(exitCode, exitReason, r.ciFinalContent, nil))
		meta := r.ciReportMeta()
		if flushErr := r.EventSink.FlushReport(meta, exitCode, exitReason, r.ciFinalContent, nil); flushErr != nil && err == nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "CI report flush failed", logging.LogOptions{Params: map[string]any{"err": flushErr.Error()}})
			err = flushErr
		}
	}()
	return r.onUserMessage(ctx, line, false)
}

func (r *Runtime) streamOptsCI(turn int) llm.StreamOpts {
	opts := llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking}
	if !r.machineMode() {
		return opts
	}
	opts.ReasoningSink = ioDiscard{}
	turnCopy := turn
	opts.OnDelta = func(channel, text string) {
		if text == "" {
			return
		}
		r.ciEmit(cievents.AssistantDelta(turnCopy, channel, text))
	}
	r.bindAPIRetry(&opts)
	return opts
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
