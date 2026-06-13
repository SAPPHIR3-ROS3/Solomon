package turnloop

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func interruptedDuringGeneration(parent, runCtx context.Context, opErr, stopErr error) bool {
	if errors.Is(context.Cause(runCtx), stopErr) {
		return true
	}
	if opErr == nil {
		return false
	}
	return parent.Err() == nil && errors.Is(runCtx.Err(), context.Canceled) && errors.Is(opErr, context.Canceled)
}

func appendSyntheticToolResults(h Host, astSeq int, invs []tooling.Invocation, toolIDs []string, start int) error {
	payload := toolingResultJSON(map[string]any{"error": h.GenerationStoppedMessage()})
	h.MutateSession(func(s *chatstore.Session) {
		for j := start; j < len(invs); j++ {
			var tm chatstore.Message
			if id := toolIDs[j]; id != "" {
				tm = chatstore.Message{Role: "tool", ToolCallID: id, Content: payload}
			} else {
				tm = chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"}
			}
			checkpoint.StampMsg(&tm, s, astSeq)
			s.Messages = append(s.Messages, tm)
			s.LastMessageAt = time.Now()
		}
	})
	return h.PersistSession()
}

func toolingResultJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal"}`
	}
	return string(b)
}
