package llm

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/openai/openai-go/v2"
	"solomon/internal/termcolor"
)

type StreamOpts struct {
	ShowThinking  bool
	ReasoningSink io.Writer
}

func StreamText(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts StreamOpts) (string, error) {
	reasonSink := opts.ReasoningSink
	if reasonSink == nil {
		reasonSink = io.Discard
	}
	stream := client.Chat.Completions.NewStreaming(ctx, params)
	var acc openai.ChatCompletionAccumulator
	var full string
	skipLeadingNL := true
	var leadBuf string
	for stream.Next() {
		ch := stream.Current()
		acc.AddChunk(ch)
		if len(ch.Choices) == 0 {
			continue
		}
		delta := ch.Choices[0].Delta
		if opts.ShowThinking {
			rs := deltaReasoningText(delta.RawJSON())
			if rs != "" {
				_, _ = io.WriteString(reasonSink, termcolor.Thinking)
				_, _ = io.WriteString(reasonSink, rs)
				_, _ = io.WriteString(reasonSink, termcolor.Reset)
			}
		}
		d := delta.Content
		if d == "" {
			continue
		}
		if skipLeadingNL {
			leadBuf += d
			t := strings.TrimLeft(leadBuf, "\n\r")
			if t == "" {
				continue
			}
			skipLeadingNL = false
			full = t
			_, _ = io.WriteString(contentOut, t)
			continue
		}
		full += d
		_, _ = io.WriteString(contentOut, d)
	}
	if err := stream.Err(); err != nil {
		if f, ok := contentOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return full, err
	}
	if f, ok := contentOut.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	return full, nil
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
