package test

import (
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"testing"
	"time"
)

func TestWorkedForIncludesToolWait(t *testing.T) {
	for _, saved := range []bool{false, true} {
		message := chatstore.Message{Role: "assistant", Content: "Done", TurnWallSecs: 2, ToolWallSecs: 12}
		if saved {
			message.TurnWallDisplay = 5
			message.TurnDisplaySaved = true
		}
		got, visible := server.ChatMessageFromSessionForTest("", "", message, []chatstore.Message{message}, 0, nil, false, "test")
		want := 14.0
		if saved {
			want = 17
		}
		if !visible || got.WorkedFor != want {
			t.Fatalf("saved=%v: got %v, visible=%v; want %v", saved, got.WorkedFor, visible, want)
		}
	}
}

func TestMessageReasoningEffortSurvivesStorageAndAPI(t *testing.T) {
	for _, effort := range []string{"none", "high", "max"} {
		original := chatstore.Message{Role: "assistant", Content: "Done", ReasoningEffort: effort}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var restored chatstore.Message
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		got, visible := server.ChatMessageFromSessionForTest("", "", restored, []chatstore.Message{restored}, 0, nil, false, "test")
		if !visible || !got.HasStats || got.ReasoningEffort != effort {
			t.Fatalf("reasoning effort %q lost: %+v", effort, got)
		}
	}
}

func TestLegacySynchronousSubagentWait(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	source := []chatstore.Message{
		{Role: "assistant", CreatedAt: start, TurnWallSecs: 6, ToolCalls: []chatstore.ToolCall{{Name: "subagent", ID: "child"}}},
		{Role: "tool", ToolCallID: "child"},
		{Role: "assistant", Content: "Done", CreatedAt: start.Add(8*time.Minute + 35*time.Second), TurnWallSecs: 5},
	}
	if got := server.MessageToolWallSecondsForTest(source[0], source, 0); got != 510 {
		t.Fatalf("wait=%v, want 510 seconds", got)
	}
	first, _ := server.ChatMessageFromSessionForTest("", "", source[0], source, 0, nil, false, "first")
	last, _ := server.ChatMessageFromSessionForTest("", "", source[2], source, 2, nil, false, "last")
	if total := first.WorkedFor + last.WorkedFor; total != 521 {
		t.Fatalf("API total=%v, want 521 seconds", total)
	}
	source[0].ToolWallSecs = 510
	if got := server.MessageToolWallSecondsForTest(source[0], source, 0); got != 510 {
		t.Fatalf("persisted wait counted twice: %v", got)
	}
	source[0].ToolWallSecs = 0
	source[1].Role = "user"
	if got := server.MessageToolWallSecondsForTest(source[0], source, 0); got != 0 {
		t.Fatalf("user idle time counted: %v", got)
	}
	source[1].Role = "tool"
	source[2].CreatedAt = time.Time{}
	if got := server.MessageToolWallSecondsForTest(source[0], source, 0); got != 0 {
		t.Fatalf("missing timestamp produced wait: %v", got)
	}
}
