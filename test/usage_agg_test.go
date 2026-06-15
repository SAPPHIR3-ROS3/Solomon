package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestAggregateConsecutiveTurnUsage_TwoTurns(t *testing.T) {
	usages := []llm.UsageStats{
		{PromptTokens: 100, ReasoningTokens: 50, ResponseTokens: 30, TotalTokens: 180},
		{PromptTokens: 200, ReasoningTokens: 40, ResponseTokens: 25, TotalTokens: 265},
	}

	agg := llm.AggregateConsecutiveTurnUsage(usages)

	t.Logf("Aggregated: Prompt=%d Cached=%d Reasoning=%d Response=%d Total=%d",
		agg.PromptTokens, agg.CachedPromptTokens, agg.ReasoningTokens, agg.ResponseTokens, agg.TotalTokens)

	if agg.TotalTokens == 0 {
		t.Error("TotalTokens is 0 after aggregation — never recalculated!")
	}

	expectedReasoning := int64(50 + 40)
	if agg.ReasoningTokens != expectedReasoning {
		t.Errorf("ReasoningTokens = %d, want %d", agg.ReasoningTokens, expectedReasoning)
	}

	expectedResponse := int64(30 + 25)
	if agg.ResponseTokens != expectedResponse {
		t.Errorf("ResponseTokens = %d, want %d", agg.ResponseTokens, expectedResponse)
	}

	if agg.PromptTokens != 200 {
		t.Errorf("PromptTokens = %d, want 200 (last turn)", agg.PromptTokens)
	}
}

func TestUsageTokensDisplayParts_MultiTurn(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "ciao, fai qualcosa"},
		{Role: "assistant", Content: "", ReasoningText: "ragionamento 1"},
		{Role: "tool", ToolCallID: "call_1", Content: `{"result":"ok"}`},
	}

	sys := "system prompt"

	agg := llm.UsageStats{
		PromptTokens:       200,
		CachedPromptTokens: 0,
		ReasoningTokens:    90,
		ResponseTokens:     55,
		TotalTokens:        345,
	}

	ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(sys, msgs, agg, 2)

	t.Logf("ctxTok=%d usrTok=%d ctxEst=%v reasonTok=%d respTok=%d totalTok=%d",
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok)

	if totalTok <= 0 {
		t.Errorf("totalTok = %d, expected positive value", totalTok)
	}

	if ctxTok < 0 {
		t.Errorf("ctxTok is negative: %d", ctxTok)
	}
	if usrTok < 0 {
		t.Errorf("usrTok is negative: %d", usrTok)
	}
}
