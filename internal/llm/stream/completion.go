package stream

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/promptparts"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/streamio"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func StreamText(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts apitype.StreamOpts) (string, apitype.UsageStats, error) {
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
	reasonSink := opts.ReasoningSink
	if reasonSink == nil {
		reasonSink = io.Discard
	}
	tStart := time.Now()
	stream := client.Chat.Completions.NewStreaming(ctx, params)
	var acc openai.ChatCompletionAccumulator
	var full string
	skipLeadingNL := true
	var leadBuf string
	var reasoningFromUsage int64
	var tTTFT time.Time
	var tFirstVisible time.Time
	var sawReasoning bool
	var printedThought bool
	var legacyStopped bool
	for stream.Next() {
		if legacyStopped {
			break
		}
		ch := stream.Current()
		if ch.JSON.Usage.Valid() {
			rt := ch.Usage.CompletionTokensDetails.ReasoningTokens
			if rt == 0 {
				rt = parseLooseReasoningTokensFromUsageRawJSON(ch.Usage.RawJSON())
			}
			reasoningFromUsage = rt
		}
		if !acc.AddChunk(ch) {
			flushStreamOut(contentOut)
			return "", apitype.UsageStats{}, streamAccumulatorRejectErr(stream)
		}
		if len(ch.Choices) == 0 {
			continue
		}
		delta := ch.Choices[0].Delta
		rs := deltaReasoningText(delta.RawJSON())
		if rs != "" {
			sawReasoning = true
			if opts.ShowThinking {
				writeReasoningDelta(reasonSink, rs)
			}
		}
		if tTTFT.IsZero() && firstAssistDelta(delta, opts) {
			tTTFT = time.Now()
		}
		if tFirstVisible.IsZero() && firstVisibleAssistDelta(delta) {
			tFirstVisible = time.Now()
			if sawReasoning && !printedThought {
				writeThoughtForLine(reasonSink, time.Since(tStart).Seconds())
				printedThought = true
			}
		}
		d := delta.Content
		if d == "" {
			continue
		}
		if skipLeadingNL {
			leadBuf += d
			t := strings.TrimLeft(leadBuf, "\n\r")
			if t == "" {
				continue
			}
			skipLeadingNL = false
			full = t
			if stopped, err := writeStreamContentLegacy(contentOut, t); err != nil {
				flushStreamOut(contentOut)
				return full, apitype.UsageStats{}, err
			} else if stopped {
				legacyStopped = true
				full = streamTruncatedContent(contentOut, full)
				break
			}
			continue
		}
		full += d
		if stopped, err := writeStreamContentLegacy(contentOut, d); err != nil {
			flushStreamOut(contentOut)
			return full, apitype.UsageStats{}, err
		} else if stopped {
			legacyStopped = true
			full = streamTruncatedContent(contentOut, full)
			break
		}
	}
	if legacyStopped {
		closeCompletionStream(stream)
	}
	if err := stream.Err(); err != nil && !(legacyStopped && legacyStreamStopErrOK(err)) {
		if f, ok := contentOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return full, apitype.UsageStats{}, err
	}
	if !legacyStopped {
		if err := flushStreamOutErr(contentOut); err != nil {
			return full, apitype.UsageStats{}, err
		}
	}
	tEnd := time.Now()
	u := buildUsageStats(acc, reasoningFromUsage, tStart, tTTFT, tFirstVisible, tEnd)
	if sawReasoning && !printedThought && u.ThoughtSecs > 0 {
		writeThoughtForLine(reasonSink, u.ThoughtSecs)
	}
	return full, u, nil
}

func StreamAssistantTurn(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts apitype.StreamOpts) (apitype.AssistantTurnResult, error) {
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
	reasonSink := opts.ReasoningSink
	if reasonSink == nil {
		reasonSink = io.Discard
	}
	tStart := time.Now()
	stream := client.Chat.Completions.NewStreaming(ctx, params)
	var acc openai.ChatCompletionAccumulator
	skipLeadingNL := true
	var leadBuf string
	var reasoningFromUsage int64
	var reasoningBuf strings.Builder
	var tTTFT time.Time
	var tFirstVisible time.Time
	var printedThought bool
	var legacyStopped bool
	var proxyToolCorrection string
	for stream.Next() {
		if legacyStopped {
			break
		}
		ch := stream.Current()
		if corr := parseProxyCorrectionFromChunkRawJSON(ch.RawJSON()); corr != "" {
			proxyToolCorrection = corr
		}
		if ev := parseCursorToolEventFromChunkRawJSON(ch.RawJSON()); ev != "" && opts.OnDelta != nil {
			opts.OnDelta("cursor_tool", ev)
		}
		if ch.JSON.Usage.Valid() {
			rt := ch.Usage.CompletionTokensDetails.ReasoningTokens
			if rt == 0 {
				rt = parseLooseReasoningTokensFromUsageRawJSON(ch.Usage.RawJSON())
			}
			reasoningFromUsage = rt
		}
		if !acc.AddChunk(ch) {
			flushStreamOut(contentOut)
			return apitype.AssistantTurnResult{}, streamAccumulatorRejectErr(stream)
		}
		if len(ch.Choices) == 0 {
			continue
		}
		delta := ch.Choices[0].Delta
		rs := deltaReasoningText(delta.RawJSON())
		if rs != "" {
			reasoningBuf.WriteString(rs)
			if opts.OnDelta != nil {
				opts.OnDelta("reasoning", rs)
			}
			if opts.ShowThinking {
				writeReasoningDelta(reasonSink, rs)
			}
		}
		if tTTFT.IsZero() && firstAssistDelta(delta, opts) {
			tTTFT = time.Now()
		}
		if tFirstVisible.IsZero() && firstVisibleAssistDelta(delta) {
			tFirstVisible = time.Now()
			if reasoningBuf.Len() > 0 && !printedThought {
				writeThoughtForLine(reasonSink, time.Since(tStart).Seconds())
				printedThought = true
			}
		}
		d := delta.Content
		if d == "" {
			continue
		}
		if skipLeadingNL {
			leadBuf += d
			t := strings.TrimLeft(leadBuf, "\n\r")
			if t == "" {
				continue
			}
			skipLeadingNL = false
			if opts.OnDelta != nil {
				opts.OnDelta("content", t)
			}
			if stopped, err := writeStreamContentLegacy(contentOut, t); err != nil {
				flushStreamOut(contentOut)
				return apitype.AssistantTurnResult{}, err
			} else if stopped {
				legacyStopped = true
				break
			}
			continue
		}
		if opts.OnDelta != nil {
			opts.OnDelta("content", d)
		}
		if stopped, err := writeStreamContentLegacy(contentOut, d); err != nil {
			flushStreamOut(contentOut)
			return apitype.AssistantTurnResult{}, err
		} else if stopped {
			legacyStopped = true
			break
		}
	}
	if legacyStopped {
		closeCompletionStream(stream)
	}
	if err := stream.Err(); err != nil && !(legacyStopped && legacyStreamStopErrOK(err)) {
		if f, ok := contentOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return apitype.AssistantTurnResult{}, err
	}
	if !legacyStopped {
		if err := flushStreamOutErr(contentOut); err != nil {
			return apitype.AssistantTurnResult{}, err
		}
	}
	tEnd := time.Now()
	var out apitype.AssistantTurnResult
	out.ReasoningText = tooling.StripLegacyToolBlocks(strings.TrimSpace(streamio.NormalizeReasoningWhitespace(reasoningBuf.String())))
	if legacyStopped {
		out.Content = streamTruncatedContent(contentOut, "")
	} else if len(acc.Choices) > 0 {
		msg := acc.Choices[0].Message
		out.Content = msg.Content
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			out.ToolCalls = append(out.ToolCalls, apitype.AssistantToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}
	out.ProxyToolCorrection = proxyToolCorrection
	out.Usage = buildUsageStats(acc, reasoningFromUsage, tStart, tTTFT, tFirstVisible, tEnd)
	if !printedThought && strings.TrimSpace(out.ReasoningText) != "" {
		writeThoughtForLine(reasonSink, out.Usage.ThoughtSecs)
	}
	return out, nil
}

func AggregateConsecutiveTurnUsage(usages []apitype.UsageStats) apitype.UsageStats {
	if len(usages) == 0 {
		return apitype.UsageStats{}
	}
	if len(usages) == 1 {
		return usages[0]
	}
	out := usages[len(usages)-1]
	out.ReasoningTokens = 0
	out.ResponseTokens = 0
	out.TotalTokens = 0
	out.OutputTPS = 0
	out.PromptTPS = 0
	out.TurnWallSecs = 0
	for _, u := range usages {
		out.ReasoningTokens += u.ReasoningTokens
		out.ResponseTokens += u.ResponseTokens
		out.TurnWallSecs += u.TurnWallSecs
		out.OutputTPS += u.OutputTPS
		out.PromptTPS += u.PromptTPS
	}
	n := float64(len(usages))
	out.OutputTPS /= n
	out.PromptTPS /= n
	out.TTFTSecs = usages[0].TTFTSecs
	out.TotalTokens = out.PromptTokens + out.ReasoningTokens + out.ResponseTokens
	return out
}

func UsageTokensDisplayParts(system string, msgs []chatstore.Message, u apitype.UsageStats, turnCount int) (contextTok, lastUserTok int64, contextEstimated bool, reasoningTok, responseTok, totalTok int64) {
	reasoningTok = u.ReasoningTokens
	responseTok = u.ResponseTokens
	contextTok, lastUserTok, contextEstimated = promptparts.UsagePromptParts(system, msgs, u.PromptTokens, u.CachedPromptTokens)
	if turnCount > 1 {
		d := reasoningTok + responseTok
		if contextTok > d {
			contextTok -= d
		} else {
			contextTok = 0
		}
		totalTok = contextTok + lastUserTok + reasoningTok + responseTok
		return
	}
	totalTok = u.TotalTokens
	if totalTok <= 0 {
		totalTok = contextTok + lastUserTok + reasoningTok + responseTok
	}
	return
}

func usageFromAccumulator(acc openai.ChatCompletionAccumulator, reasoningTok int64) apitype.UsageStats {
	comp := acc.Usage.CompletionTokens
	resp := comp - reasoningTok
	if resp < 0 {
		resp = 0
	}
	return apitype.UsageStats{
		PromptTokens:       acc.Usage.PromptTokens,
		CachedPromptTokens: acc.Usage.PromptTokensDetails.CachedTokens,
		ReasoningTokens:    reasoningTok,
		ResponseTokens:     resp,
		TotalTokens:        acc.Usage.TotalTokens,
	}
}

func buildUsageStats(acc openai.ChatCompletionAccumulator, reasoningTok int64, tStart, tTTFT, tFirstVisible, tEnd time.Time) apitype.UsageStats {
	u := usageFromAccumulator(acc, reasoningTok)
	u.TurnWallSecs = tEnd.Sub(tStart).Seconds()
	if !tFirstVisible.IsZero() {
		u.ThoughtSecs = tFirstVisible.Sub(tStart).Seconds()
	} else if reasoningTok > 0 {
		u.ThoughtSecs = u.TurnWallSecs
	}
	if tTTFT.IsZero() {
		if reasoningTok > 0 {
			u.TTFTSecs = u.TurnWallSecs
			genDur := u.TurnWallSecs
			outToks := u.ResponseTokens + u.ReasoningTokens
			if genDur > 0 && outToks > 0 {
				u.OutputTPS = float64(outToks) / genDur
			}
			if u.TTFTSecs > 0 && u.PromptTokens > 0 {
				u.PromptTPS = float64(u.PromptTokens) / u.TTFTSecs
			}
		}
		return u
	}
	u.TTFTSecs = tTTFT.Sub(tStart).Seconds()
	genDur := tEnd.Sub(tTTFT).Seconds()
	outToks := u.ResponseTokens + u.ReasoningTokens
	if genDur > 0 && outToks > 0 {
		u.OutputTPS = float64(outToks) / genDur
	}
	if u.TTFTSecs > 0 && u.PromptTokens > 0 {
		u.PromptTPS = float64(u.PromptTokens) / u.TTFTSecs
	}
	return u
}
