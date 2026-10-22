package termcolor

import (
	"strings"
	"testing"
)

func TestUsageCacheUnavailableAndZero(t *testing.T) {
	line := UsageTokensLine(10, 1, 0, 2, 13, -1, 0, 1, 0, 0, false, 1)
	if !strings.Contains(line, "cache read:n/a write:0") {
		t.Fatalf("unknown and zero conflated: %q", line)
	}
}
