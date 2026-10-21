package chatstore

import "testing"

func TestIsCompactionSummary(t *testing.T) {
	for _, tc := range []struct {
		name, role, content string
		want                bool
	}{
		{"stored", "assistant", "---\n[Conversation summary]\n---\n\nSummary\n---", true},
		{"wide separator", "assistant", "========\n[Conversation summary]\n========\n\nSummary", true},
		{"mention", "assistant", "Il blocco [Conversation summary] contiene la sintesi.", false},
		{"quoted envelope", "assistant", "Ecco il formato:\n---\n[Conversation summary]\n---\nSummary", false},
		{"tool output", "tool", "---\n[Conversation summary]\n---\nSummary", false},
		{"unframed marker", "assistant", "[Conversation summary]\nOrdinary explanation", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCompactionSummary(Message{Role: tc.role, Content: tc.content}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
