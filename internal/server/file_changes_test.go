package server

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"testing"
)

func TestFileChangesAPIKeepsTurnSnapshots(t *testing.T) {
	messages := []chatstore.Message{{Role: "user", Content: "Edit"}, {Role: "assistant", Content: "Done", FileChanges: []chatstore.FileChangeSummary{{Path: "a.txt", Added: 1, AddedLines: []string{"own line"}}}}, {Role: "user", Content: "Explain"}, {Role: "assistant", Content: "Explanation"}}
	chat := apiChatFromSession("project", &chatstore.Session{ID: "own-chat", Messages: messages}, nil)
	if len(chat.Messages) != 4 || len(chat.Messages[1].FileChanges) != 1 || chat.Messages[1].FileChanges[0].Path != "a.txt" || len(chat.Messages[3].FileChanges) != 0 {
		t.Fatalf("API snapshots: %+v", chat.Messages)
	}
	other := apiChatFromSession("project", &chatstore.Session{ID: "other-chat", Messages: []chatstore.Message{{Role: "assistant", Content: "Other"}}}, nil)
	if len(other.Messages[0].FileChanges) != 0 {
		t.Fatal("file changes leaked to another chat")
	}
}
