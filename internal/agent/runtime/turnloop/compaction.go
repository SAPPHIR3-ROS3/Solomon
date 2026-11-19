package turnloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
)

func cursorPromptOverhead(system string, tools []llm.ToolDef) int64 {
	definitions, _ := json.Marshal(tools)
	return tokcount.TextTokens(system, tokcount.DefaultModel) + tokcount.TextTokens(string(definitions), tokcount.DefaultModel)
}

func compactBeforeRequest(ctx context.Context, h Host, msgs []chatstore.Message, contextEstimate int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	threshold := h.CompactionThreshold()
	if threshold <= 0 {
		return false, nil
	}
	lastAssistant := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && (contextEstimate == 0 || !chatstore.IsCompactionSummary(msgs[i])) {
			lastAssistant = i
			break
		}
	}
	if lastAssistant < 0 {
		return false, nil
	}
	promptTokens := msgs[lastAssistant].PromptTokens
	if contextEstimate > 0 {
		promptTokens = contextEstimate
		model, _, _ := strings.Cut(strings.TrimPrefix(h.ModelName(), "cursor-"), "[")
		if model == "grok-4.7" || strings.HasPrefix(model, "grok-4.7-") {
			threshold = min(threshold, 256*1024*4/5)
		}
	}
	if promptTokens < threshold {
		return false, nil
	}
	end := len(msgs)
	for end > 0 && msgs[end-1].Role == "user" {
		end--
	}
	for _, call := range msgs[lastAssistant].ToolCalls {
		found := false
		for _, msg := range msgs[lastAssistant+1 : end] {
			if msg.Role == "tool" && msg.ToolCallID == call.ID {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	pendingStart := end
	if contextEstimate > 0 && end == len(msgs) {
		for i := end - 1; i >= 0; i-- {
			if msgs[i].Role == "user" {
				pendingStart = i
				break
			}
		}
		if pendingStart == end {
			return false, nil
		}
	}
	deps := h.SlashDeps(ctx)
	deps.MutateSession = func(fn func(*chatstore.Session)) {
		fn(&chatstore.Session{Messages: msgs[:end]})
	}
	body, err := commands.SummarizeBody(deps)
	if err != nil {
		return false, fmt.Errorf("auto-compact: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var chatID string
	h.MutateSession(func(s *chatstore.Session) {
		chatID = s.ID
		pending := append([]chatstore.Message(nil), s.Messages[end:]...)
		if pendingStart < end {
			pending = append([]chatstore.Message{s.Messages[pendingStart]}, pending...)
		}
		chatstore.ApplyCompactionWithPendingMessages(s, body, time.Now(), pending)
	})
	if err := h.PersistSession(); err != nil {
		return false, fmt.Errorf("auto-compact: %w", err)
	}
	if h.MachineMode() {
		h.CIEmit(cievents.ContextCompacted(deps.ProjHex, chatID))
	}
	commands.PrintSystem(h.Out(), "context summarized")
	return true, nil
}
