package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/streamio"
	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

type streamState struct {
	content     strings.Builder
	reasoning   strings.Builder
	toolNames   map[int]string
	toolArgs    map[int]*strings.Builder
	toolIDs     map[int]string
	toolInitial map[int]string
	usage       UsagePayload
	stopReason  string
	stopped     bool
	thinking    map[int]contentBlock
}

func (b *Backend) postStream(ctx context.Context, body map[string]any) (*ssestream.Stream[sdk.MessageStreamEventUnion], error) {
	params, err := sdkParams(body)
	if err != nil {
		return nil, err
	}
	opts := []option.RequestOption{option.WithHeader("Accept", "text/event-stream")}
	if b.auth.OAuth() {
		opts = append(opts, option.WithQuery("beta", "true"), option.WithHeader("anthropic-beta", OAuthBeta), option.WithHeader("x-stainless-helper-method", "stream"))
	}
	client := b.client()
	return client.Messages.NewStreaming(ctx, params, opts...), nil
}

func maxTokens(cfg *config.Root) int64 {
	if cfg != nil && cfg.MaxResponseTokens > 0 {
		return int64(cfg.MaxResponseTokens)
	}
	return 8192
}

func (b *Backend) buildBody(req apitype.TurnRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": maxTokens(req.Cfg),
		"messages":   buildMessages(req.Messages, req.ImageFiles),
		"stream":     stream,
	}
	if s := strings.TrimSpace(req.System); s != "" {
		body["system"] = s
	}
	if tools := buildTools(req.Tools); len(tools) > 0 {
		body["tools"] = tools
	}
	shapeOAuthBody(body, req, b.auth)
	return body
}

func (b *Backend) buildSimpleBody(req apitype.SimpleCompletionRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": maxTokens(req.Cfg),
		"stream":     stream,
		"messages": []messageParam{
			{Role: "user", Content: []contentBlock{{"type": "text", "text": req.User}}},
		},
	}
	if s := strings.TrimSpace(req.System); s != "" {
		body["system"] = s
	}
	shapeOAuthSimpleBody(body, req, b.auth)
	return body
}

func (b *Backend) StreamTurn(ctx context.Context, req apitype.TurnRequest, contentOut io.Writer, opts apitype.StreamOpts) (apitype.AssistantTurnResult, error) {
	current, err := b.forRequest(ctx)
	if err != nil {
		return apitype.AssistantTurnResult{}, err
	}
	b = current
	resp, err := b.postStream(ctx, b.buildBody(req, true))
	if err != nil {
		return apitype.AssistantTurnResult{}, err
	}
	defer resp.Close()
	turn, err := readStreamTurn(resp, contentOut, opts)
	if b.auth.OAuth() {
		restoreOAuthToolNames(&turn, req.Tools)
	}
	return turn, err
}

func (b *Backend) StreamText(ctx context.Context, req apitype.SimpleCompletionRequest, contentOut io.Writer, opts apitype.StreamOpts) (string, apitype.UsageStats, error) {
	current, err := b.forRequest(ctx)
	if err != nil {
		return "", apitype.UsageStats{}, err
	}
	b = current
	resp, err := b.postStream(ctx, b.buildSimpleBody(req, true))
	if err != nil {
		return "", apitype.UsageStats{}, err
	}
	defer resp.Close()
	turn, err := readStreamTurn(resp, contentOut, opts)
	if err != nil {
		return "", apitype.UsageStats{}, err
	}
	return turn.Content, turn.Usage, nil
}

func readStreamTurn(stream *ssestream.Stream[sdk.MessageStreamEventUnion], contentOut io.Writer, opts apitype.StreamOpts) (apitype.AssistantTurnResult, error) {
	reasonSink := opts.ReasoningSink
	if reasonSink == nil {
		reasonSink = io.Discard
	}
	tStart := time.Now()
	st := &streamState{toolNames: map[int]string{}, toolArgs: map[int]*strings.Builder{}, toolIDs: map[int]string{}, thinking: map[int]contentBlock{}, toolInitial: map[int]string{}}
	var tTTFT, tFirstVisible time.Time
	var legacyStopped bool
	for stream.Next() {
		ev := stream.Current()
		if err := applyStreamEvent(st, ev, contentOut, reasonSink, opts, &tTTFT, &tFirstVisible, tStart); err != nil {
			if errors.Is(err, streamio.ErrLegacyEarlyStop) {
				legacyStopped = true
				break
			}
			return apitype.AssistantTurnResult{}, err
		}
		if st.stopped {
			break
		}
	}
	if !legacyStopped {
		if err := stream.Err(); err != nil {
			return apitype.AssistantTurnResult{}, claude.SDKError(err)
		}
		if !st.stopped {
			return apitype.AssistantTurnResult{}, fmt.Errorf("Anthropic stream ended before message_stop: %w", io.ErrUnexpectedEOF)
		}
	}
	var out apitype.AssistantTurnResult
	out.Content = streamio.TruncatedContent(contentOut, strings.TrimSpace(st.content.String()))
	thinkingIndices := make([]int, 0, len(st.thinking))
	for index := range st.thinking {
		thinkingIndices = append(thinkingIndices, index)
	}
	sort.Ints(thinkingIndices)
	for _, index := range thinkingIndices {
		block := st.thinking[index]
		if block["type"] == "thinking" && block["signature"] == "" {
			continue
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return apitype.AssistantTurnResult{}, err
		}
		out.AnthropicThinking = append(out.AnthropicThinking, raw)
	}
	out.ReasoningText = strings.TrimSpace(streamio.NormalizeReasoningWhitespace(st.reasoning.String()))
	out.FinishReason = normalizeFinishReason(st.stopReason)
	out.Usage = NormalizeUsage(st.usage)
	fillTiming(&out.Usage, tStart, tTTFT, tFirstVisible, time.Now())
	if !legacyStopped {
		indices := make([]int, 0, len(st.toolNames))
		for idx := range st.toolNames {
			indices = append(indices, idx)
		}
		sort.Ints(indices)
		for _, idx := range indices {
			name := st.toolNames[idx]
			args := st.toolInitial[idx]
			if argB := st.toolArgs[idx]; argB != nil && argB.Len() > 0 {
				args = argB.String()
			}
			if !json.Valid([]byte(args)) {
				return apitype.AssistantTurnResult{}, fmt.Errorf("Anthropic tool %s returned invalid JSON arguments", name)
			}
			out.ToolCalls = append(out.ToolCalls, apitype.AssistantToolCall{
				ID:        st.toolIDs[idx],
				Name:      name,
				Arguments: args,
			})
		}
	}
	if out.FinishReason == "" && len(out.ToolCalls) > 0 {
		out.FinishReason = apitype.FinishReasonToolCalls
	}
	return out, nil
}

func normalizeFinishReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "end_turn", "stop_sequence":
		return apitype.FinishReasonStop
	case "tool_use":
		return apitype.FinishReasonToolCalls
	case "max_tokens":
		return apitype.FinishReasonLength
	default:
		return strings.TrimSpace(reason)
	}
}

func applyStreamEvent(st *streamState, ev sdk.MessageStreamEventUnion, contentOut, reasonSink io.Writer, opts apitype.StreamOpts, tTTFT, tFirstVisible *time.Time, tStart time.Time) error {
	switch ev.Type {
	case "message_start":
		u := ev.Message.Usage
		st.usage = UsagePayload{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, CacheReadInputTokens: u.CacheReadInputTokens, CacheCreationInputTokens: u.CacheCreationInputTokens}
	case "message_delta":
		u := ev.Usage
		if u.JSON.OutputTokens.Valid() {
			st.usage.OutputTokens = u.OutputTokens
		}
		if u.JSON.InputTokens.Valid() {
			st.usage.InputTokens = u.InputTokens
		}
		if u.JSON.CacheReadInputTokens.Valid() {
			st.usage.CacheReadInputTokens = u.CacheReadInputTokens
		}
		if u.JSON.CacheCreationInputTokens.Valid() {
			st.usage.CacheCreationInputTokens = u.CacheCreationInputTokens
		}
		if ev.Delta.StopReason != "" {
			st.stopReason = string(ev.Delta.StopReason)
		}
	case "message_stop":
		st.stopped = true
	case "content_block_start":
		if ev.ContentBlock.Type == "thinking" || ev.ContentBlock.Type == "redacted_thinking" {
			var block contentBlock
			if err := json.Unmarshal([]byte(ev.ContentBlock.RawJSON()), &block); err != nil {
				return err
			}
			st.thinking[int(ev.Index)] = block
		}
		if ev.ContentBlock.Type == "tool_use" {
			idx := int(ev.Index)
			st.toolNames[idx] = ev.ContentBlock.Name
			st.toolIDs[idx] = ev.ContentBlock.ID
			st.toolArgs[idx] = &strings.Builder{}
			if ev.ContentBlock.Input != nil {
				raw, err := json.Marshal(ev.ContentBlock.Input)
				if err != nil {
					return err
				}
				st.toolInitial[idx] = string(raw)
			} else {
				st.toolInitial[idx] = "{}"
			}
		}
	case "content_block_delta":
		switch ev.Delta.Type {
		case "text_delta":
			if text := ev.Delta.Text; text != "" {
				if tTTFT.IsZero() {
					*tTTFT = time.Now()
				}
				if tFirstVisible.IsZero() {
					*tFirstVisible = time.Now()
				}
				if opts.OnDelta != nil {
					opts.OnDelta("content", text)
				}
				st.content.WriteString(text)
				if stopped, err := streamio.WriteContentLegacy(contentOut, text); err != nil {
					return err
				} else if stopped {
					return streamio.ErrLegacyEarlyStop
				}
			}
		case "input_json_delta":
			if args := st.toolArgs[int(ev.Index)]; args != nil {
				args.WriteString(ev.Delta.PartialJSON)
			}
		case "signature_delta":
			if block := st.thinking[int(ev.Index)]; block != nil {
				current, _ := block["signature"].(string)
				block["signature"] = current + ev.Delta.Signature
			}
		case "thinking_delta":
			if block := st.thinking[int(ev.Index)]; block != nil {
				current, _ := block["thinking"].(string)
				block["thinking"] = current + ev.Delta.Thinking
			}
			if text := ev.Delta.Thinking; text != "" {
				if tTTFT.IsZero() {
					*tTTFT = time.Now()
				}
				st.reasoning.WriteString(text)
				if opts.OnDelta != nil {
					opts.OnDelta("reasoning", text)
				}
				if opts.ShowThinking {
					streamio.WriteReasoningDelta(reasonSink, text)
				}
			}
		}
	}
	return nil
}

func fillTiming(u *apitype.UsageStats, tStart, tTTFT, tFirstVisible, tEnd time.Time) {
	u.TurnWallSecs = tEnd.Sub(tStart).Seconds()
	if !tFirstVisible.IsZero() {
		u.ThoughtSecs = tFirstVisible.Sub(tStart).Seconds()
	}
	if !tTTFT.IsZero() {
		u.TTFTSecs = tTTFT.Sub(tStart).Seconds()
		genDur := tEnd.Sub(tTTFT).Seconds()
		outToks := u.ResponseTokens + u.ReasoningTokens
		if genDur > 0 && outToks > 0 {
			u.OutputTPS = float64(outToks) / genDur
		}
		if u.TTFTSecs > 0 && u.PromptTokens > 0 {
			u.PromptTPS = float64(u.PromptTokens) / u.TTFTSecs
		}
	}
}
