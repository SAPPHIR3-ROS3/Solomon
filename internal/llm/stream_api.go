package llm

import (
	"context"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/stream"
	"github.com/openai/openai-go/v2"
)

var ErrStreamAccumulatorRejected = stream.ErrStreamAccumulatorRejected

func StreamText(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	return stream.StreamText(ctx, client, params, contentOut, opts)
}

func StreamAssistantTurn(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	return stream.StreamAssistantTurn(ctx, client, params, contentOut, opts)
}

func AggregateConsecutiveTurnUsage(usages []UsageStats) UsageStats {
	return stream.AggregateConsecutiveTurnUsage(usages)
}

func UsageTokensDisplayParts(system string, msgs []chatstore.Message, u UsageStats, turnCount int) (contextTok, lastUserTok int64, contextEstimated bool, reasoningTok, responseTok, totalTok int64) {
	return stream.UsageTokensDisplayParts(system, msgs, u, turnCount)
}

func ParseCursorToolEventFromChunkRawJSON(raw string) string {
	return stream.ParseCursorToolEventFromChunkRawJSON(raw)
}
