package agentruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func (r *Runtime) printToolLine(cpSeq int, branchKey, name string, rawArgs json.RawMessage) {
	if cpSeq > 0 {
		if intent := tooling.ExtractToolIntent(rawArgs); intent != "" {
			fmt.Fprintf(r.Out, "%s%s\n", checkpoint.FormatCheckpointPrefix(cpSeq, branchKey), termcolor.WrapThinking(intent))
		}
	}
	tooling.WriteToolDisplayLines(r.Out, cpSeq, branchKey, formatToolDisplayLines(name, rawArgs))
}

func (r *Runtime) streamOptsWithRetry(showThinking bool, reasonSink io.Writer) llm.StreamOpts {
	opts := llm.StreamOpts{ShowThinking: showThinking, ReasoningSink: reasonSink}
	if r.cursorNativeToolsEnabled() {
		out := r.Out
		opts.OnDelta = func(channel, text string) {
			if channel == "cursor_tool" && out != nil && !r.machineMode() {
				r.printCursorNativeToolEvent(text)
			}
		}
	}
	r.bindAPIRetry(&opts)
	return opts
}

func (r *Runtime) bindAPIRetry(opts *llm.StreamOpts) {
	if opts == nil {
		return
	}
	out := r.Out
	opts.OnRetry = func(attempt, max int, err error, wait time.Duration) {
		line := llm.RetryMessage(attempt, max, err, wait)
		if out != nil && !r.machineMode() {
			fmt.Fprintf(out, "\n%s\n", termcolor.WrapRed(line))
			flushWriter(out)
			return
		}
		logging.Log(logging.WARNING_LOG_LEVEL, line, logging.LogOptions{Params: map[string]any{
			"attempt": attempt,
			"max":     max,
			"wait_ms": wait.Milliseconds(),
		}})
	}
}
