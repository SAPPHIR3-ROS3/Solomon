package turnloop

import (
	"fmt"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func printTurnUsage(h Host, out io.Writer, system string, messages []chatstore.Message, usages []llm.UsageStats, contextEstimate int64) {
	agg := llm.AggregateConsecutiveTurnUsage(usages)
	ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(system, messages, agg)
	line := termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.CachedPromptTokens, agg.CacheCreationPromptTokens, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs)
	if contextEstimate > 0 {
		line += fmt.Sprintf("\tcontext estimate: %d", contextEstimate)
	}
	fmt.Fprintln(out, line)
	saveMainMessageChange(h, "flushUsageStats", func(s *chatstore.Session) int {
		return chatstore.ApplyTurnUsageDisplayToLastAssistant(s, ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok, agg.CachedPromptTokens, agg.CacheCreationPromptTokens, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, agg.TurnWallSecs)
	})
}
