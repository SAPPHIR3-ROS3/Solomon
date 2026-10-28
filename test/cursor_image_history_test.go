package test

import (
	"encoding/base64"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

func TestCursorToolImageReplay(t *testing.T) {
	attachment := images.Attachment{Path: "/image.png", MIMEType: "image/png", Data: []byte("image snapshot")}
	msgs := []chatstore.Message{
		{Role: "user", Content: "read screenshot"},
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "one", Name: "orchestrate"}, {ID: "two", Name: "orchestrate"}}},
		{Role: "tool", ToolCallID: "one", Content: "image read", ToolImages: []images.Attachment{attachment}},
		{Role: "tool", ToolCallID: "two", Content: "other result"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "follow up"},
	}
	history, err := llm.CursorAgentHistoryForTest(msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 6 {
		t.Fatalf("history messages = %d", len(history))
	}
	var imageMsg struct {
		Role    string
		Content []map[string]any
	}
	if err := json.Unmarshal(history[4], &imageMsg); err != nil {
		t.Fatal(err)
	}
	if imageMsg.Role != "user" || len(imageMsg.Content) != 2 || imageMsg.Content[1]["type"] != "image" || imageMsg.Content[1]["image"] != base64.StdEncoding.EncodeToString(attachment.Data) {
		t.Fatalf("incorrect replay image: %s", history[4])
	}
	if !strings.Contains(string(history[2]), `"toolCallId":"one"`) || !strings.Contains(string(history[3]), `"toolCallId":"two"`) {
		t.Fatal("image interrupted tool batch")
	}
}
