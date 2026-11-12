package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
)

func TestWrappedSyntaxRestoresColorAfterGutter(t *testing.T) {
	pink := "\x1b[38;2;227;148;220m"
	lines := syntax.RestoreWrappedStyles([]string{pink + "first", "second", "third\x1b[0m tail"})
	if !strings.HasPrefix(lines[1], pink) || !strings.HasPrefix(lines[2], pink) {
		t.Fatal("continued string lost its color")
	}
	if !strings.HasSuffix(lines[0], "\x1b[0m") || !strings.HasSuffix(lines[1], "\x1b[0m") {
		t.Fatal("color leaked into next line gutter")
	}
	if !strings.Contains(lines[2], "third\x1b[0m tail") {
		t.Fatal("string end did not restore following code")
	}
}
func TestWrappedSyntaxZeroRGBIsNotReset(t *testing.T) {
	color := "\x1b[38;2;0;128;0m"
	lines := syntax.RestoreWrappedStyles([]string{color + "first", "second\x1b[0m"})
	if !strings.HasPrefix(lines[1], color) {
		t.Fatal("zero RGB component incorrectly reset the style")
	}
}
