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

func TestWorkedForDoesNotRepeatSavedTurnWall(t *testing.T) {
	start := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	source := []chatstore.Message{
		{Role: "assistant", CreatedAt: start, TurnWallSecs: 10, ToolWallSecs: 20, ToolCalls: []chatstore.ToolCall{{Name: "shell", ID: "a"}}},
		{Role: "tool", ToolCallID: "a"},
		{Role: "assistant", Content: "Done", CreatedAt: start.Add(30 * time.Second), TurnWallSecs: 8, TurnWallDisplay: 18, TurnDisplaySaved: true},
	}
	first, visible := server.ChatMessageFromSessionForTest("", "", source[0], source, 0, nil, false, "first")
	last, lastVisible := server.ChatMessageFromSessionForTest("", "", source[2], source, 2, nil, false, "last")
	if !visible || !lastVisible || first.WorkedFor != 20 || last.WorkedFor != 18 || first.WorkedFor+last.WorkedFor != 38 {
		t.Fatalf("first=%v last=%v visible=%v/%v; want 20 and 18", first.WorkedFor, last.WorkedFor, visible, lastVisible)
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

func TestWorkedForUsesElapsedTurnAcrossMidnight(t *testing.T) {
	start := time.Date(2026, 11, 19, 23, 51, 12, 0, time.UTC)
	elapsed := 39*time.Minute + 17*time.Second
	source := []chatstore.Message{
		{Role: "user", CreatedAt: start, Content: "Go"},
		{Role: "assistant", CreatedAt: start.Add(20 * time.Second), TurnWallSecs: 20, ToolWallSecs: 3},
		{Role: "user", GUIHidden: true, CreatedAt: start.Add(39 * time.Minute), Content: "[subagent-completed] Done"},
		{Role: "assistant", CreatedAt: start.Add(elapsed), TurnWallDisplay: 45, Content: "Done"},
		{Role: "user", CreatedAt: start.Add(time.Hour), Content: "Next"},
	}
	first, _ := server.ChatMessageFromSessionForTest("", "", source[1], source, 1, nil, false, "first")
	last, _ := server.ChatMessageFromSessionForTest("", "", source[3], source, 3, nil, false, "last")
	if first.WorkedFor != 0 || last.WorkedFor != elapsed.Seconds() {
		t.Fatalf("worked for: first=%v last=%v; want first=0 last=%v", first.WorkedFor, last.WorkedFor, elapsed.Seconds())
	}
}
