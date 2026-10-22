package cursor

import "testing"

func TestAgentCacheCounterPresence(t *testing.T) {
	if got := agentWireCacheCount(nil, 3); got != -1 {
		t.Fatalf("absent: %d", got)
	}
	if got := agentWireCacheCount([]agentWireField{{number: 3, wire: 0, value: 0}}, 3); got != 0 {
		t.Fatalf("explicit zero: %d", got)
	}
}
