package test

import (
	"testing"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
)

func TestResolveAgentModelID(t *testing.T) {
	for _, tc := range []struct{ selected, want string }{
		{"cursor-grok-4.7", "grok-4.7"},
		{"cursor-grok-4.7-high", "grok-4.7-high"},
		{"cursor-grok-4.6-medium-fast", "grok-4.6-medium-fast"},
		{"grok-4.7", "grok-4.7"},
		{"composer-2.5", "composer-2.5"},
		{"cursor-other-model", "cursor-other-model"},
		{"  cursor-grok-4.7  ", "grok-4.7"},
	} {
		t.Run(tc.selected, func(t *testing.T) {
			if got := cursorauth.ResolveAgentModelID(tc.selected); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
