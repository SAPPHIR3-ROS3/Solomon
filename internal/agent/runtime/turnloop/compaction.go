package turnloop

import (
	"context"
	"fmt"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func compactBeforeRequest(ctx context.Context, h Host) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	threshold := h.CompactionThreshold()
	if threshold <= 0 {
		return nil
	}
	msgs, _ := h.SessionMessagesSnapshot()
	lastAssistant := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			lastAssistant = i
			break
		}
	}
	if lastAssistant < 0 || msgs[lastAssistant].PromptTokens < threshold {
		return nil
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
			return nil
		}
	}
	deps := h.SlashDeps(ctx)
	deps.MutateSession = func(fn func(*chatstore.Session)) {
		fn(&chatstore.Session{Messages: msgs[:end]})
	}
	body, err := commands.SummarizeBody(deps)
	if err != nil {
		return fmt.Errorf("auto-compact: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var chatID string
	h.MutateSession(func(s *chatstore.Session) {
		chatID = s.ID
		pending := append([]chatstore.Message(nil), s.Messages[end:]...)
		chatstore.ApplyCompactionWithPendingMessages(s, body, time.Now(), pending)
	})
	if err := h.PersistSession(); err != nil {
		return fmt.Errorf("auto-compact: %w", err)
	}
	if h.MachineMode() {
		h.CIEmit(cievents.ContextCompacted(deps.ProjHex, chatID))
	}
	commands.PrintSystem(h.Out(), "context summarized")
	return nil
}
