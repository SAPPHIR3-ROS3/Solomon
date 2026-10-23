package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

type bufferedToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type bufferedToolCall struct {
	Index    int                  `json:"-"`
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function bufferedToolFunction `json:"function"`
}

type bufferedChatMessage struct {
	Role                      string             `json:"role"`
	Content                   string             `json:"content"`
	ToolCalls                 []bufferedToolCall `json:"tool_calls,omitempty"`
	ReasoningContent          string             `json:"reasoning_content,omitempty"`
	ReasoningEncryptedContent string             `json:"reasoning_encrypted_content,omitempty"`
	ReasoningItem             string             `json:"reasoning_item,omitempty"`
}

type bufferedChatCompletion struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []bufferedChoice `json:"choices"`
	Usage   map[string]any   `json:"usage"`
}

type bufferedChoice struct {
	Index        int                 `json:"index"`
	FinishReason string              `json:"finish_reason"`
	Message      bufferedChatMessage `json:"message"`
}

func BufferChatCompletionFromCodexSSE(body io.Reader, model string) ([]byte, error) {
	transformer := newSSETransformer(model)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var dataLines [][]byte
	var responseID, streamModel, role, finishReason string
	var reasoningContent, reasoningEncrypted, reasoningItem string
	var created int64
	var usage map[string]any
	var contentBuilder bytes.Buffer
	toolCalls := map[int]*bufferedToolCall{}
	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		raw := bytes.Join(dataLines, []byte("\n"))
		dataLines = dataLines[:0]
		out, _, err := transformer.transform(raw)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return nil
		}
		for _, line := range bytes.Split(out, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var chunk struct {
				ID      string         `json:"id"`
				Created int64          `json:"created"`
				Model   string         `json:"model"`
				Usage   map[string]any `json:"usage"`
				Choices []struct {
					Delta struct {
						Role                      string `json:"role"`
						Content                   string `json:"content"`
						ReasoningContent          string `json:"reasoning_content"`
						ReasoningEncryptedContent string `json:"reasoning_encrypted_content"`
						ReasoningItem             string `json:"reasoning_item"`
						ToolCalls                 []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}
			if responseID == "" && chunk.ID != "" {
				responseID = chunk.ID
			}
			if streamModel == "" && chunk.Model != "" {
				streamModel = chunk.Model
			}
			if created == 0 && chunk.Created != 0 {
				created = chunk.Created
			}
			if len(chunk.Usage) > 0 {
				usage = chunk.Usage
			}
			for _, ch := range chunk.Choices {
				if ch.Delta.Role != "" && role == "" {
					role = ch.Delta.Role
				}
				contentBuilder.WriteString(ch.Delta.Content)
				reasoningContent += ch.Delta.ReasoningContent
				if ch.Delta.ReasoningEncryptedContent != "" {
					reasoningEncrypted = ch.Delta.ReasoningEncryptedContent
				}
				if ch.Delta.ReasoningItem != "" {
					reasoningItem = ch.Delta.ReasoningItem
				}
				for _, delta := range ch.Delta.ToolCalls {
					call := toolCalls[delta.Index]
					if call == nil {
						call = &bufferedToolCall{Index: delta.Index}
						toolCalls[delta.Index] = call
					}
					if delta.ID != "" {
						call.ID = delta.ID
					}
					if delta.Type != "" {
						call.Type = delta.Type
					}
					if delta.Function.Name != "" {
						call.Function.Name = delta.Function.Name
					}
					call.Function.Arguments += delta.Function.Arguments
				}
				if ch.FinishReason != nil && *ch.FinishReason != "" {
					finishReason = *ch.FinishReason
				}
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			if err := flushEvent(); err != nil {
				return nil, err
			}
			continue
		}
		if bytes.HasPrefix(trimmed, []byte(":")) {
			continue
		}
		if bytes.HasPrefix(trimmed, []byte("data:")) {
			payload := bytes.TrimPrefix(trimmed, []byte("data:"))
			if len(payload) > 0 && payload[0] == ' ' {
				payload = payload[1:]
			}
			if bytes.Equal(bytes.TrimSpace(payload), []byte("[DONE]")) {
				if err := flushEvent(); err != nil {
					return nil, err
				}
				if !transformer.completed {
					return nil, io.ErrUnexpectedEOF
				}
				continue
			}
			cp := make([]byte, len(payload))
			copy(cp, payload)
			dataLines = append(dataLines, cp)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan codex SSE: %w", err)
	}
	if err := flushEvent(); err != nil {
		return nil, err
	}
	if !transformer.completed {
		return nil, io.ErrUnexpectedEOF
	}
	if responseID == "" {
		responseID = "chatcmpl-buffered"
	}
	if created == 0 {
		created = time.Now().Unix()
	}
	if streamModel != "" {
		model = streamModel
	}
	if role == "" {
		role = "assistant"
	}
	if finishReason == "" {
		finishReason = "stop"
	}
	orderedCalls := make([]bufferedToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		orderedCalls = append(orderedCalls, *call)
	}
	sort.Slice(orderedCalls, func(i, j int) bool { return orderedCalls[i].Index < orderedCalls[j].Index })
	for i := range orderedCalls {
		orderedCalls[i].Index = 0
	}
	if usage == nil {
		usage = map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
	}
	resp := bufferedChatCompletion{
		ID: responseID, Object: "chat.completion", Created: created, Model: model, Usage: usage,
		Choices: []bufferedChoice{{
			Index: 0, FinishReason: finishReason,
			Message: bufferedChatMessage{
				Role: role, Content: contentBuilder.String(), ToolCalls: orderedCalls,
				ReasoningContent: reasoningContent, ReasoningEncryptedContent: reasoningEncrypted, ReasoningItem: reasoningItem,
			},
		}},
	}
	return json.Marshal(resp)
}
