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
	if r.cursorNativeToolsEnabled() && reasonSink != io.Discard {
		opts.OnDelta = func(channel, text string) {
			if channel == "cursor_tool" && reasonSink != nil && !r.machineMode() {
				r.printCursorNativeToolEvent(text)
			}
		}
	}
	r.bindAPIRetryTo(&opts, reasonSink)
	return opts
}

func (r *Runtime) bindAPIRetry(opts *llm.StreamOpts) {
	r.bindAPIRetryTo(opts, r.Out)
}

func (r *Runtime) bindAPIRetryTo(opts *llm.StreamOpts, out io.Writer) {
	if opts == nil {
		return
	}
	opts.OnRetry = func(attempt, max int, err error, wait time.Duration) {
		line := llm.RetryMessage(attempt, max, err, wait)
		if out != nil && out != io.Discard && !r.machineMode() {
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
