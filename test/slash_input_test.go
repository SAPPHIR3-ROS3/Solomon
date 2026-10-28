package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
)

func TestSlashInputDoesNotTreatAbsolutePathsAsCommands(t *testing.T) {
	for _, tc := range []struct {
		line    string
		command bool
	}{
		{"/help", true},
		{"  /agent  ", true},
		{"/docs /home/oni/file.png", true},
		{"/skill:review inspect /tmp/file", true},
		{"/custom.skill arguments", true},
		{"/unknown-command", true},
		{"/home/oni/Pictures/Screenshots/Screenshot 2026-10-28.png", false},
		{"  /tmp/screenshot.webp explain this", false},
		{"/home/oni/photo.jpeg", false},
		{"/", false},
		{"//server/share/image.png", false},
		{"Read /tmp/image.png", false},
		{"", false},
	} {
		t.Run(tc.line, func(t *testing.T) {
			if got := slash.IsCommand(tc.line); got != tc.command {
				t.Fatalf("IsCommand(%q) = %v, want %v", tc.line, got, tc.command)
			}
		})
	}
}
