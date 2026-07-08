package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
)

type hubSink struct {
	buf *eventBuffer
	hub *Hub
	mu  sync.Mutex
}

func newHubSink(hub *Hub, buf *eventBuffer) *hubSink {
	return &hubSink{hub: hub, buf: buf}
}

func (s *hubSink) Emit(ev cievents.Event) {
	for _, re := range cieventToResponseEvents(ev) {
		id := s.buf.append(re.Type, re.Data)
		re.ID = id
		s.hub.Publish(re)
	}
}

func (s *hubSink) StreamMode() bool { return true }

func (s *hubSink) Events() []cievents.Event { return nil }

func (s *hubSink) FlushReport(cievents.ReportMeta, int, string, string, any) error {
	return nil
}

func writeSSE(w io.Writer, id int, typ string, data map[string]any) error {
	payload := map[string]any{"type": typ}
	for k, v := range data {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if id > 0 {
		if _, err := fmt.Fprintf(w, "id: %d\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, b); err != nil {
		return err
	}
	if f, ok := w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func replayBuffer(w io.Writer, buf *eventBuffer, after int) error {
	for _, ev := range buf.since(after) {
		if err := writeSSE(w, ev.ID, ev.Type, ev.Data); err != nil {
			return err
		}
	}
	return nil
}

func replayEvents(w io.Writer, events []streamEvent, after int) error {
	for _, ev := range events {
		if ev.ID <= after {
			continue
		}
		if err := writeSSE(w, ev.ID, ev.Type, ev.Data); err != nil {
			return err
		}
	}
	return nil
}

func streamFromHub(ctx context.Context, w io.Writer, hub *Hub, buf *eventBuffer, after int) error {
	if err := replayBuffer(w, buf, after); err != nil {
		return err
	}
	ch, stop := hub.Subscribe()
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if ev.ID <= after {
				continue
			}
			if err := writeSSE(w, ev.ID, ev.Type, ev.Data); err != nil {
				return err
			}
			after = ev.ID
			if ev.Type == "response.completed" || ev.Type == "response.failed" || ev.Type == "response.cancelled" || ev.Type == "error" {
				return nil
			}
		}
	}
}

func extractInputText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("input is required")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", fmt.Errorf("invalid input")
	}
	var b strings.Builder
	for _, it := range items {
		if role, _ := it["role"].(string); role != "user" {
			continue
		}
		switch c := it["content"].(type) {
		case string:
			b.WriteString(c)
		case []any:
			for _, p := range c {
				m, _ := p.(map[string]any)
				if t, _ := m["text"].(string); t != "" {
					b.WriteString(t)
				}
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func metadataSlashPayload(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	if v, ok := meta["slash"].(map[string]any); ok {
		return v
	}
	return meta
}

func slashOutputResponse(id, conversation, text string) map[string]any {
	return map[string]any{
		"id":           id,
		"object":       "response",
		"status":       "completed",
		"conversation": conversation,
		"output": []map[string]any{{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{{
				"type": "output_text",
				"text": text,
			}},
		}},
		"output_text": text,
	}
}

func collectSlashOutput(w io.Writer) string {
	if w == nil {
		return ""
	}
	if b, ok := w.(*bytes.Buffer); ok {
		return strings.TrimSpace(b.String())
	}
	return ""
}
