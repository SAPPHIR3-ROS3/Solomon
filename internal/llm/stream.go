package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

var ErrStreamAccumulatorRejected = errors.New("chat completion stream accumulator rejected chunk")

func flushStreamOut(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

func streamAccumulatorRejectErr(stream interface{ Err() error }) error {
	if err := stream.Err(); err != nil {
		return err
	}
	return ErrStreamAccumulatorRejected
}

func parseLooseReasoningTokensFromUsageRawJSON(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return 0
	}
	tryNum := func(msg json.RawMessage) int64 {
		var n int64
		if json.Unmarshal(msg, &n) == nil && n > 0 {
			return n
		}
		var f float64
		if json.Unmarshal(msg, &f) == nil && f > 0 {
			return int64(f + 0.5)
		}
		return 0
	}
	if v, ok := top["reasoning_tokens"]; ok {
		if n := tryNum(v); n > 0 {
			return n
		}
	}
	if v, ok := top["completion_tokens_details"]; ok {
		var det map[string]json.RawMessage
		if json.Unmarshal(v, &det) != nil {
			return 0
		}
		if r, ok := det["reasoning_tokens"]; ok {
			if n := tryNum(r); n > 0 {
				return n
			}
		}
	}
	return 0
}

type AssistantToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type UsageStats struct {
	PromptTokens       int64
	CachedPromptTokens int64
	ReasoningTokens    int64
	ResponseTokens     int64
	TotalTokens        int64
	OutputTPS          float64
	TTFTSecs           float64
	PromptTPS          float64
	TurnWallSecs       float64
	ThoughtSecs        float64
}

type AssistantTurnResult struct {
	Content        string
	ReasoningText  string
	ToolCalls      []AssistantToolCall
	Usage          UsageStats
}

type StreamOpts struct {
	ShowThinking  bool
	ReasoningSink io.Writer
}

func StreamText(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
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
	for stream.Next() {
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
			return "", UsageStats{}, streamAccumulatorRejectErr(stream)
		}
		if len(ch.Choices) == 0 {
			continue
		}
		delta := ch.Choices[0].Delta
		rs := deltaReasoningText(delta.RawJSON())
		if rs != "" {
			sawReasoning = true
			if opts.ShowThinking {
				_, _ = io.WriteString(reasonSink, termcolor.WrapThinking(rs))
			}
		}
		if tTTFT.IsZero() && firstAssistDelta(delta, opts) {
			tTTFT = time.Now()
		}
		if tFirstVisible.IsZero() && firstVisibleAssistDelta(delta) {
			tFirstVisible = time.Now()
			if sawReasoning && !printedThought {
				_, _ = fmt.Fprintf(reasonSink, "%s\n", termcolor.ThoughtForSuffix(time.Since(tStart).Seconds()))
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
			_, _ = io.WriteString(contentOut, t)
			continue
		}
		full += d
		_, _ = io.WriteString(contentOut, d)
	}
	if err := stream.Err(); err != nil {
		if f, ok := contentOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return full, UsageStats{}, err
	}
	if f, ok := contentOut.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	tEnd := time.Now()
	u := buildUsageStats(acc, reasoningFromUsage, tStart, tTTFT, tFirstVisible, tEnd)
	if sawReasoning && !printedThought && u.ThoughtSecs > 0 {
		_, _ = fmt.Fprintf(reasonSink, "%s\n", termcolor.ThoughtForSuffix(u.ThoughtSecs))
	}
	return full, u, nil
}

func StreamAssistantTurn(ctx context.Context, client openai.Client, params openai.ChatCompletionNewParams, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
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
	for stream.Next() {
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
			return AssistantTurnResult{}, streamAccumulatorRejectErr(stream)
		}
		if len(ch.Choices) == 0 {
			continue
		}
		delta := ch.Choices[0].Delta
		rs := deltaReasoningText(delta.RawJSON())
		if rs != "" {
			reasoningBuf.WriteString(rs)
			if opts.ShowThinking {
				_, _ = io.WriteString(reasonSink, termcolor.WrapThinking(rs))
			}
		}
		if tTTFT.IsZero() && firstAssistDelta(delta, opts) {
			tTTFT = time.Now()
		}
		if tFirstVisible.IsZero() && firstVisibleAssistDelta(delta) {
			tFirstVisible = time.Now()
			if reasoningBuf.Len() > 0 && !printedThought {
				_, _ = fmt.Fprintf(reasonSink, "%s\n", termcolor.ThoughtForSuffix(time.Since(tStart).Seconds()))
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
			_, _ = io.WriteString(contentOut, t)
			continue
		}
		_, _ = io.WriteString(contentOut, d)
	}
	if err := stream.Err(); err != nil {
		if f, ok := contentOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		return AssistantTurnResult{}, err
	}
	if f, ok := contentOut.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	tEnd := time.Now()
	var out AssistantTurnResult
	out.ReasoningText = strings.TrimSpace(reasoningBuf.String())
	if len(acc.Choices) > 0 {
		msg := acc.Choices[0].Message
		out.Content = msg.Content
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			out.ToolCalls = append(out.ToolCalls, AssistantToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}
	out.Usage = buildUsageStats(acc, reasoningFromUsage, tStart, tTTFT, tFirstVisible, tEnd)
	if !printedThought && strings.TrimSpace(out.ReasoningText) != "" {
		_, _ = fmt.Fprintf(reasonSink, "%s\n", termcolor.ThoughtForSuffix(out.Usage.ThoughtSecs))
	}
	return out, nil
}

func AggregateConsecutiveTurnUsage(usages []UsageStats) UsageStats {
	if len(usages) == 0 {
		return UsageStats{}
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

func UsageTokensDisplayParts(system string, msgs []chatstore.Message, u UsageStats, turnCount int) (contextTok, lastUserTok int64, contextEstimated bool, reasoningTok, responseTok, totalTok int64) {
	reasoningTok = u.ReasoningTokens
	responseTok = u.ResponseTokens
	contextTok, lastUserTok, contextEstimated = UsagePromptParts(system, msgs, u.PromptTokens, u.CachedPromptTokens)
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

func usageFromAccumulator(acc openai.ChatCompletionAccumulator, reasoningTok int64) UsageStats {
	comp := acc.Usage.CompletionTokens
	resp := comp - reasoningTok
	if resp < 0 {
		resp = 0
	}
	return UsageStats{
		PromptTokens:       acc.Usage.PromptTokens,
		CachedPromptTokens: acc.Usage.PromptTokensDetails.CachedTokens,
		ReasoningTokens:    reasoningTok,
		ResponseTokens:     resp,
		TotalTokens:        acc.Usage.TotalTokens,
	}
}

func buildUsageStats(acc openai.ChatCompletionAccumulator, reasoningTok int64, tStart, tTTFT, tFirstVisible, tEnd time.Time) UsageStats {
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

func firstAssistDelta(delta openai.ChatCompletionChunkChoiceDelta, opts StreamOpts) bool {
	if strings.TrimSpace(delta.Content) != "" {
		return true
	}
	if strings.TrimSpace(delta.Refusal) != "" {
		return true
	}
	for _, tc := range delta.ToolCalls {
		if tc.Function.Name != "" || tc.Function.Arguments != "" {
			return true
		}
	}
	if opts.ShowThinking && deltaReasoningText(delta.RawJSON()) != "" {
		return true
	}
	return false
}

func firstVisibleAssistDelta(delta openai.ChatCompletionChunkChoiceDelta) bool {
	if strings.TrimSpace(delta.Content) != "" {
		return true
	}
	if strings.TrimSpace(delta.Refusal) != "" {
		return true
	}
	for _, tc := range delta.ToolCalls {
		if tc.Function.Name != "" || tc.Function.Arguments != "" {
			return true
		}
	}
	return false
}

func deltaReasoningText(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return ""
	}
	keys := []string{"reasoning_content", "reasoning", "thinking", "thought"}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
		var obj map[string]any
		if json.Unmarshal(v, &obj) != nil || len(obj) == 0 {
			continue
		}
		t, ok := obj["text"].(string)
		if ok && t != "" {
			return t
		}
	}
	return ""
}
