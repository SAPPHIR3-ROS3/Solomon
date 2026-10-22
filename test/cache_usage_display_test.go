package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"strings"
	"testing"
)

func TestUsageCacheUnavailableAndZero(t *testing.T) {
	line := termcolor.UsageTokensLine(10, 1, 0, 2, 13, -1, 0, 1, 0, 0, false, 1)
	if !strings.Contains(line, "cache read:n/a write:0") {
		t.Fatalf("unknown and zero conflated: %q", line)
	}
}
