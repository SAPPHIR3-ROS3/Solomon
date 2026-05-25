package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func TestStoredUsageLineForTurnRangeSavedDisplay(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", TurnDisplaySaved: true, TurnContextTokens: 80, TurnUserTokens: 10, TurnReasonTokens: 4, TurnRespTokens: 6, TurnTotalDisplay: 100, TurnOutputTPS: 12.5, TurnTTFTSecs: 1.2, TurnPromptTPS: 100, TurnWallDisplay: 3.5},
	}
	_, usr, r, resp, total, outTPS, ttft, pp, wall, _, ok := chatstore.StoredUsageLineForTurnRange(msgs, 1, len(msgs))
	if !ok {
		t.Fatal("expected usage")
	}
	if usr != 10 || r != 4 || resp != 6 || total != 100 {
		t.Fatalf("tokens usr=%d r=%d resp=%d total=%d", usr, r, resp, total)
	}
	if outTPS != 12.5 || ttft != 1.2 || pp != 100 || wall != 3.5 {
		t.Fatalf("timing out=%v ttft=%v pp=%v wall=%v", outTPS, ttft, pp, wall)
	}
}
