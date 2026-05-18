package llm

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func TestAggregateConsecutiveTurnUsage_TwoTurns(t *testing.T) {
	// Simula 2 turni consecutivi con tool calling.
	// Turno 1: prompt=100, reasoning=50, response=30, total=180
	// Turno 2: prompt=200 (include risposta turno 1), reasoning=40, response=25, total=265
	usages := []UsageStats{
		{PromptTokens: 100, ReasoningTokens: 50, ResponseTokens: 30, TotalTokens: 180},
		{PromptTokens: 200, ReasoningTokens: 40, ResponseTokens: 25, TotalTokens: 265},
	}

	agg := AggregateConsecutiveTurnUsage(usages)

	t.Logf("Aggregated: Prompt=%d Cached=%d Reasoning=%d Response=%d Total=%d",
		agg.PromptTokens, agg.CachedPromptTokens, agg.ReasoningTokens, agg.ResponseTokens, agg.TotalTokens)

	// TotalTokens dovrebbe essere ricalcolato, non restare 0
	if agg.TotalTokens == 0 {
		t.Error("TotalTokens is 0 after aggregation — never recalculated!")
	}

	// Reasoning e Response dovrebbero essere sommati
	expectedReasoning := int64(50 + 40)
	if agg.ReasoningTokens != expectedReasoning {
		t.Errorf("ReasoningTokens = %d, want %d", agg.ReasoningTokens, expectedReasoning)
	}

	expectedResponse := int64(30 + 25)
	if agg.ResponseTokens != expectedResponse {
		t.Errorf("ResponseTokens = %d, want %d", agg.ResponseTokens, expectedResponse)
	}

	// PromptTokens dovrebbe essere quello dell'ultimo turno (già include tutto il contesto)
	if agg.PromptTokens != 200 {
		t.Errorf("PromptTokens = %d, want 200 (last turn)", agg.PromptTokens)
	}
}

func TestUsageTokensDisplayParts_MultiTurn(t *testing.T) {
	// Simula msgs con user + assistant(tool call) + tool result + ultimo turno
	msgs := []chatstore.Message{
		{Role: "user", Content: "ciao, fai qualcosa"},
		{Role: "assistant", Content: "", ReasoningText: "ragionamento 1"},
		{Role: "tool", ToolCallID: "call_1", Content: `{"result":"ok"}`},
	}

	sys := "system prompt"

	// Aggregato di 2 turni
	agg := UsageStats{
		PromptTokens:    200,
		CachedPromptTokens: 0,
		ReasoningTokens: 90, // 50+40
		ResponseTokens:  55, // 30+25
		TotalTokens:     0,  // non ricalcolato da AggregateConsecutiveTurnUsage
	}

	ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := UsageTokensDisplayParts(sys, msgs, agg, 2)

	t.Logf("ctxTok=%d usrTok=%d ctxEst=%v reasonTok=%d respTok=%d totalTok=%d",
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok)

	// Il totale dovrebbe essere positivo e coerente
	if totalTok <= 0 {
		t.Errorf("totalTok = %d, expected positive value", totalTok)
	}

	// Verifica che la sottrazione non produca valori negativi
	if ctxTok < 0 {
		t.Errorf("ctxTok is negative: %d", ctxTok)
	}
	if usrTok < 0 {
		t.Errorf("usrTok is negative: %d", usrTok)
	}
}
