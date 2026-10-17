package server

import (
	"encoding/json"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestMessageStatsPreserveRecordedTarget(t *testing.T) {
	// Exercise the persisted transcript format before constructing the API view.
	var session chatstore.Session
	if err := json.Unmarshal([]byte(`{"messages":[{"role":"assistant","content":"hello","provider":"custom-provider","model":"recorded-model"}]}`), &session); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &session); err != nil {
		t.Fatal(err)
	}
	message, ok := apiMessageFromSession("project", "chat", session.Messages[0], session.Messages, 0, nil, false, "a1")
	if !ok || message.Stats == nil {
		t.Fatal("recorded target must expose message info even without usage")
	}
	if message.Stats.Provider != "custom-provider" || message.Stats.Model != "recorded-model" {
		t.Fatalf("stats lost recorded target: %+v", message.Stats)
	}
	legacy := apiStatsFromMessage(chatstore.Message{Role: "assistant", ResponseTokens: 5})
	if legacy == nil || legacy.Provider != "" || legacy.Model != "" || legacy.ResponseTokens != 5 {
		t.Fatalf("legacy stats: %+v", legacy)
	}
}
