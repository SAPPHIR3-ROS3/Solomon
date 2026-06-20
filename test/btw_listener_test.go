package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/btw/listener"
)

func TestParseBtwSubmit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"/btw what is this?", "what is this?", true},
		{"/btw  spaced  ", "spaced", true},
		{"/summarize", "", false},
		{"/btw", "", false},
		{"/btw ", "", false},
	}
	for _, tc := range cases {
		got, ok := listener.ParseSubmit(tc.line)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseSubmit(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}
