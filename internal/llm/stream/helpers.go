package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/streamio"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/openai/openai-go/v2"
)

var ErrStreamAccumulatorRejected = errors.New("chat completion stream accumulator rejected chunk")

func flushStreamOut(w io.Writer) {
	_ = flushStreamOutErr(w)
}

func flushStreamOutErr(w io.Writer) error {
	if f, ok := w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func writeThoughtForLine(sink io.Writer, secs float64) {
	_, _ = fmt.Fprintf(sink, "\n%s\n", termcolor.ThoughtForSuffix(secs))
}

func writeStreamContentLegacy(w io.Writer, s string) (legacyStopped bool, err error) {
	return streamio.WriteContentLegacy(w, s)
}

func streamTruncatedContent(w io.Writer, fallback string) string {
	return streamio.TruncatedContent(w, fallback)
}

func writeReasoningDelta(sink io.Writer, s string) {
	streamio.WriteReasoningDelta(sink, s)
}

func closeCompletionStream(stream any) {
	if c, ok := stream.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

func legacyStreamStopErrOK(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, sub := range []string{
		"forcibly closed",
		"unexpected eof",
		"context canceled",
		"operation was canceled",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func streamAccumulatorRejectErr(stream interface{ Err() error }) error {
	if err := stream.Err(); err != nil {
		return err
	}
	return ErrStreamAccumulatorRejected
}

func parseLooseReasoningTokensFromUsageRawJSON(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return 0
	}
	tryNum := func(msg json.RawMessage) int64 {
		var n int64
		if json.Unmarshal(msg, &n) == nil && n > 0 {
			return n
		}
		var f float64
		if json.Unmarshal(msg, &f) == nil && f > 0 {
			return int64(f + 0.5)
		}
		return 0
	}
	if v, ok := top["reasoning_tokens"]; ok {
		if n := tryNum(v); n > 0 {
			return n
		}
	}
	if v, ok := top["completion_tokens_details"]; ok {
		var det map[string]json.RawMessage
		if json.Unmarshal(v, &det) != nil {
			return 0
		}
		if r, ok := det["reasoning_tokens"]; ok {
			if n := tryNum(r); n > 0 {
				return n
			}
		}
	}
	return 0
}

func parseProxyCorrectionFromChunkRawJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return ""
	}
	v, ok := top["solomon_proxy_correction"]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func ParseCursorToolEventFromChunkRawJSON(raw string) string {
	return parseCursorToolEventFromChunkRawJSON(raw)
}

func parseCursorToolEventFromChunkRawJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return ""
	}
	v, ok := top["solomon_cursor_tool_event"]
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(v))
}

func deltaReasoningText(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return ""
	}
	keys := []string{"reasoning_content", "reasoning", "thinking", "thought"}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
		var obj map[string]any
		if json.Unmarshal(v, &obj) != nil || len(obj) == 0 {
			continue
		}
		t, ok := obj["text"].(string)
		if ok && t != "" {
			return t
		}
	}
	return ""
}

func firstAssistDelta(delta openai.ChatCompletionChunkChoiceDelta, opts apitype.StreamOpts) bool {
	if strings.TrimSpace(delta.Content) != "" {
		return true
	}
	if strings.TrimSpace(delta.Refusal) != "" {
		return true
	}
	for _, tc := range delta.ToolCalls {
		if tc.Function.Name != "" || tc.Function.Arguments != "" {
			return true
		}
	}
	if opts.ShowThinking && deltaReasoningText(delta.RawJSON()) != "" {
		return true
	}
	return false
}

func firstVisibleAssistDelta(delta openai.ChatCompletionChunkChoiceDelta) bool {
	if strings.TrimSpace(delta.Content) != "" {
		return true
	}
	if strings.TrimSpace(delta.Refusal) != "" {
		return true
	}
	for _, tc := range delta.ToolCalls {
		if tc.Function.Name != "" || tc.Function.Arguments != "" {
			return true
		}
	}
	return false
}
