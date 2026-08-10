package chatstore

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
)

type turnUsageCall struct {
	promptTokens       int64
	cachedPromptTokens int64
	reasoningTokens    int64
	responseTokens     int64
	totalTokens        int64
	outputTPS          float64
	ttftSecs           float64
	promptTPS          float64
	turnWallSecs       float64
	userPromptTokens   int64
}

func BackfillSessionAssistantUsage(msgs []Message) {
	for i := range msgs {
		if msgs[i].Role != "assistant" {
			continue
		}
		BackfillAssistantUsageFromTextIfEmpty(&msgs[i], msgs[:i])
	}
}

func ApplyTurnUsageDisplayToLastAssistant(s *Session, ctxTok, usrTok int64, ctxEst bool, reasonTok, respTok, totalTok int64, outputTPS, ttftSecs, promptTPS, turnWallSecs float64) {
	if s == nil {
		return
	}
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role != "assistant" {
			continue
		}
		m := &s.Messages[i]
		m.TurnDisplaySaved = true
		m.TurnContextTokens = ctxTok
		m.TurnContextEst = ctxEst
		m.TurnUserTokens = usrTok
		m.TurnReasonTokens = reasonTok
		m.TurnRespTokens = respTok
		m.TurnTotalDisplay = totalTok
		m.TurnOutputTPS = outputTPS
		m.TurnTTFTSecs = ttftSecs
		m.TurnPromptTPS = promptTPS
		m.TurnWallDisplay = turnWallSecs
		return
	}
}

func turnUsageCallFromMessage(m Message) turnUsageCall {
	return turnUsageCall{
		promptTokens:       m.PromptTokens,
		cachedPromptTokens: m.CachedPromptTokens,
		reasoningTokens:    m.ReasoningTokens,
		responseTokens:     m.ResponseTokens,
		totalTokens:        m.TurnTotalTokens,
		outputTPS:          m.OutputTPS,
		ttftSecs:           m.TTFTSecs,
		promptTPS:          m.PromptTPS,
		turnWallSecs:       m.TurnWallSecs,
		userPromptTokens:   m.UserPromptTokens,
	}
}

func aggregateTurnUsageCalls(calls []turnUsageCall) (turnUsageCall, bool) {
	if len(calls) == 0 {
		return turnUsageCall{}, false
	}
	if len(calls) == 1 {
		return calls[0], true
	}
	out := calls[len(calls)-1]
	out.reasoningTokens = 0
	out.responseTokens = 0
	out.totalTokens = 0
	out.outputTPS = 0
	out.promptTPS = 0
	out.turnWallSecs = 0
	for _, c := range calls {
		out.reasoningTokens += c.reasoningTokens
		out.responseTokens += c.responseTokens
		out.turnWallSecs += c.turnWallSecs
		out.outputTPS += c.outputTPS
		out.promptTPS += c.promptTPS
	}
	n := float64(len(calls))
	out.outputTPS /= n
	out.promptTPS /= n
	out.ttftSecs = calls[0].ttftSecs
	out.totalTokens = out.promptTokens + out.reasoningTokens + out.responseTokens
	return out, true
}

func usageDisplayPartsFromAggregated(msgs []Message, agg turnUsageCall, nCalls int) (contextTok, lastUserTok int64, contextEstimated bool, reasoningTok, responseTok, totalTok int64) {
	reasoningTok = agg.reasoningTokens
	responseTok = agg.responseTokens
	contextTok, lastUserTok, contextEstimated = usagePromptPartsFromMessages(msgs, agg.promptTokens, agg.cachedPromptTokens)
	if lastUserTok <= 0 && agg.userPromptTokens > 0 {
		lastUserTok = agg.userPromptTokens
	}
	if nCalls > 1 {
		d := reasoningTok + responseTok
		if contextTok > d {
			contextTok -= d
		} else {
			contextTok = 0
		}
		totalTok = contextTok + lastUserTok + reasoningTok + responseTok
		return
	}
	totalTok = agg.totalTokens
	if totalTok <= 0 {
		totalTok = contextTok + lastUserTok + reasoningTok + responseTok
	}
	return
}

func usagePromptPartsFromMessages(msgs []Message, promptTokens int64, cachedPromptTokens int64) (contextTok int64, lastUserTok int64, contextEstimated bool) {
	if promptTokens <= 0 {
		return 0, 0, false
	}
	if cachedPromptTokens > 0 {
		cached := cachedPromptTokens
		if cached > promptTokens {
			cached = promptTokens
		}
		return cached, promptTokens - cached, false
	}
	ctx, usr := promptDisplaySplitFromMessages(msgs, promptTokens)
	return ctx, usr, true
}

func promptDisplaySplitFromMessages(msgs []Message, apiPromptTokens int64) (contextTok int64, lastUserTok int64) {
	idx := lastUserMessageIndex(msgs)
	contextW, userW := promptWireWeights(msgs, idx, tokcount.DefaultModel)
	if idx < 0 {
		return apiPromptTokens, 0
	}
	totalW := contextW + userW
	if totalW <= 0 {
		return apiPromptTokens, 0
	}
	contextTok = apiPromptTokens * contextW / totalW
	lastUserTok = apiPromptTokens - contextTok
	return contextTok, lastUserTok
}

func lastUserMessageIndex(msgs []Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return i
		}
	}
	return -1
}

func lastAssistantMessageIndex(msgs []Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return i
		}
	}
	return -1
}

func MessageWireWeight(m Message, includeReasoning bool, model string) int64 {
	tpm, _ := tokcount.MessageOverhead(model)
	var n int64
	n += int64(tpm)
	n += tokcount.TextTokens(m.Role, model)
	content := m.Content
	if strings.TrimSpace(m.APIContent) != "" {
		content = m.APIContent
	}
	n += tokcount.TextTokens(content, model)
	if includeReasoning {
		n += tokcount.TextTokens(m.ReasoningText, model)
	}
	n += tokcount.TextTokens(m.ToolCallID, model)
	for _, tc := range m.ToolCalls {
		n += tokcount.TextTokens(tc.ID, model)
		n += tokcount.TextTokens(tc.Name, model)
		n += tokcount.TextTokens(tc.Arguments, model)
	}
	return n
}

func promptWireWeights(msgs []Message, lastUserIdx int, model string) (contextW int64, userW int64) {
	if lastUserIdx < 0 {
		lastAsst := lastAssistantMessageIndex(msgs)
		for i, m := range msgs {
			contextW += MessageWireWeight(m, i == lastAsst, model)
		}
		return contextW, 0
	}
	lastAsst := lastAssistantMessageIndex(msgs)
	for i, m := range msgs {
		if i == lastUserIdx {
			continue
		}
		contextW += MessageWireWeight(m, i == lastAsst, model)
	}
	userW = MessageWireWeight(msgs[lastUserIdx], false, model)
	return contextW, userW
}

func StoredUsageLineForTurnRange(msgs []Message, start, end int) (contextTok, lastUserTok, reasoningTok, responseTok, totalTok int64, outputTPS, ttftSecs, promptTPS, turnWallSecs float64, contextEstimated bool, ok bool) {
	if start < 0 || end > len(msgs) || start >= end {
		return 0, 0, 0, 0, 0, 0, 0, 0, 0, false, false
	}
	for i := end - 1; i >= start; i-- {
		m := msgs[i]
		if m.Role != "assistant" || !m.TurnDisplaySaved {
			continue
		}
		return m.TurnContextTokens, m.TurnUserTokens, m.TurnReasonTokens, m.TurnRespTokens, m.TurnTotalDisplay,
			m.TurnOutputTPS, m.TurnTTFTSecs, m.TurnPromptTPS, m.TurnWallDisplay, m.TurnContextEst, true
	}
	var calls []turnUsageCall
	for i := start; i < end; i++ {
		if msgs[i].Role != "assistant" {
			continue
		}
		m := msgs[i]
		if !assistantMessageHasStoredUsage(m) {
			continue
		}
		calls = append(calls, turnUsageCallFromMessage(m))
	}
	agg, found := aggregateTurnUsageCalls(calls)
	if !found {
		return 0, 0, 0, 0, 0, 0, 0, 0, 0, false, false
	}
	turnMsgs := append(append([]Message(nil), msgs[:start]...), msgs[start:end]...)
	ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := usageDisplayPartsFromAggregated(turnMsgs, agg, len(calls))
	return ctxTok, usrTok, reasonTok, respTok, totalTok, agg.outputTPS, agg.ttftSecs, agg.promptTPS, agg.turnWallSecs, ctxEst, true
}
