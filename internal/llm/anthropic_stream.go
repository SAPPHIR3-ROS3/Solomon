package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

type anthropicStreamState struct {
	content    strings.Builder
	reasoning  strings.Builder
	toolNames  map[int]string
	toolArgs   map[int]*strings.Builder
	toolIDs    map[int]string
	usage      anthropicUsagePayload
	stopReason string
}

func (b *AnthropicBackend) postStream(ctx context.Context, body map[string]any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL(b.baseURL), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", b.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	req.Header.Set("Accept", "text/event-stream")
	cli := &http.Client{Timeout: 0}
	return cli.Do(req)
}

func anthropicMaxTokens(cfg *config.Root) int64 {
	if cfg != nil && cfg.MaxResponseTokens > 0 {
		return int64(cfg.MaxResponseTokens)
	}
	return 8192
}

func (b *AnthropicBackend) buildBody(req TurnRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": anthropicMaxTokens(req.Cfg),
		"messages":   buildAnthropicMessages(req.Messages, req.ImageFiles),
		"stream":     stream,
	}
	if s := strings.TrimSpace(req.System); s != "" {
		body["system"] = s
	}
	if tools := buildAnthropicTools(req.Tools); len(tools) > 0 {
		body["tools"] = tools
	}
	return body
}

func (b *AnthropicBackend) buildSimpleBody(req SimpleCompletionRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": anthropicMaxTokens(req.Cfg),
		"stream":     stream,
		"messages": []anthropicMessageParam{
			{Role: "user", Content: []anthropicContentBlock{{"type": "text", "text": req.User}}},
		},
	}
	if s := strings.TrimSpace(req.System); s != "" {
		body["system"] = s
	}
	return body
}

func (b *AnthropicBackend) StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	resp, err := b.postStream(ctx, b.buildBody(req, true))
	if err != nil {
		return AssistantTurnResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return AssistantTurnResult{}, fmt.Errorf("anthropic API: %s: %s", resp.Status, strings.TrimSpace(string(bb)))
	}
	return readAnthropicStreamTurn(resp.Body, contentOut, opts)
}

func (b *AnthropicBackend) StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	resp, err := b.postStream(ctx, b.buildSimpleBody(req, true))
	if err != nil {
		return "", UsageStats{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", UsageStats{}, fmt.Errorf("anthropic API: %s: %s", resp.Status, strings.TrimSpace(string(bb)))
	}
	turn, err := readAnthropicStreamTurn(resp.Body, contentOut, opts)
	if err != nil {
		return "", UsageStats{}, err
	}
	return turn.Content, turn.Usage, nil
}

func readAnthropicStreamTurn(body io.Reader, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	reasonSink := opts.ReasoningSink
	if reasonSink == nil {
		reasonSink = io.Discard
	}
	tStart := time.Now()
	st := &anthropicStreamState{toolNames: map[int]string{}, toolArgs: map[int]*strings.Builder{}, toolIDs: map[int]string{}}
	var tTTFT, tFirstVisible time.Time
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var dataBuf string
	flushEvent := func() error {
		if dataBuf == "" || dataBuf == "[DONE]" {
			dataBuf = ""
			return nil
		}
		var ev map[string]json.RawMessage
		if err := json.Unmarshal([]byte(dataBuf), &ev); err != nil {
			dataBuf = ""
			return nil
		}
		dataBuf = ""
		return applyAnthropicStreamEvent(st, ev, contentOut, reasonSink, opts, &tTTFT, &tFirstVisible, tStart)
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") {
			_ = flushEvent()
			dataBuf = strings.TrimPrefix(line, "data: ")
			continue
		}
		if line == "" {
			if err := flushEvent(); err != nil {
				return AssistantTurnResult{}, err
			}
		}
	}
	_ = flushEvent()
	var out AssistantTurnResult
	out.Content = strings.TrimSpace(st.content.String())
	out.ReasoningText = strings.TrimSpace(st.reasoning.String())
	out.Usage = normalizeAnthropicUsage(st.usage)
	fillAnthropicTiming(&out.Usage, tStart, tTTFT, tFirstVisible, time.Now())
	for idx, name := range st.toolNames {
		args := ""
		if argB := st.toolArgs[idx]; argB != nil {
			args = argB.String()
		}
		out.ToolCalls = append(out.ToolCalls, AssistantToolCall{
			ID:        st.toolIDs[idx],
			Name:      name,
			Arguments: args,
		})
	}
	return out, nil
}

func applyAnthropicStreamEvent(st *anthropicStreamState, ev map[string]json.RawMessage, contentOut, reasonSink io.Writer, opts StreamOpts, tTTFT, tFirstVisible *time.Time, tStart time.Time) error {
	var typ string
	if v, ok := ev["type"]; ok {
		_ = json.Unmarshal(v, &typ)
	}
	switch typ {
	case "message_start":
		var wrap struct {
			Message struct {
				Usage anthropicUsagePayload `json:"usage"`
			} `json:"message"`
		}
		if raw, ok := ev["message"]; ok {
			_ = json.Unmarshal(raw, &wrap.Message)
		} else {
			_ = json.Unmarshal(anthropicMustMarshal(ev), &wrap)
		}
		st.usage.InputTokens += wrap.Message.Usage.InputTokens
		st.usage.CacheReadInputTokens += wrap.Message.Usage.CacheReadInputTokens
		st.usage.CacheCreationInputTokens += wrap.Message.Usage.CacheCreationInputTokens
	case "message_delta":
		var wrap struct {
			Usage anthropicUsagePayload `json:"usage"`
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		_ = json.Unmarshal(anthropicMustMarshal(ev), &wrap)
		st.usage.OutputTokens = wrap.Usage.OutputTokens
		if wrap.Usage.InputTokens > 0 {
			st.usage.InputTokens = wrap.Usage.InputTokens
		}
		st.usage.CacheReadInputTokens = wrap.Usage.CacheReadInputTokens
		st.usage.CacheCreationInputTokens = wrap.Usage.CacheCreationInputTokens
		st.stopReason = wrap.Delta.StopReason
	case "content_block_start":
		var wrap struct {
			Index   int `json:"index"`
			Content struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
		}
		_ = json.Unmarshal(anthropicMustMarshal(ev), &wrap)
		if wrap.Content.Type == "tool_use" {
			st.toolNames[wrap.Index] = wrap.Content.Name
			st.toolIDs[wrap.Index] = wrap.Content.ID
			st.toolArgs[wrap.Index] = &strings.Builder{}
		}
	case "content_block_delta":
		var wrap struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				Thinking    string `json:"thinking"`
			} `json:"delta"`
		}
		_ = json.Unmarshal(anthropicMustMarshal(ev), &wrap)
		switch wrap.Delta.Type {
		case "text_delta":
			if wrap.Delta.Text != "" {
				if tTTFT.IsZero() {
					*tTTFT = time.Now()
				}
				if tFirstVisible.IsZero() {
					*tFirstVisible = time.Now()
				}
				if opts.OnDelta != nil {
					opts.OnDelta("content", wrap.Delta.Text)
				}
				st.content.WriteString(wrap.Delta.Text)
				_, _ = io.WriteString(contentOut, wrap.Delta.Text)
			}
		case "input_json_delta":
			if argB := st.toolArgs[wrap.Index]; argB != nil {
				argB.WriteString(wrap.Delta.PartialJSON)
			}
		case "thinking_delta":
			if wrap.Delta.Thinking != "" {
				st.reasoning.WriteString(wrap.Delta.Thinking)
				if opts.ShowThinking {
					_, _ = io.WriteString(reasonSink, termcolor.WrapThinking(wrap.Delta.Thinking))
				}
			}
		}
	}
	return nil
}

func anthropicMustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func fillAnthropicTiming(u *UsageStats, tStart, tTTFT, tFirstVisible, tEnd time.Time) {
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
