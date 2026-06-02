package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type bufferedChatCompletion struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func BufferChatCompletionFromCodexSSE(body io.Reader, model string) ([]byte, error) {
	transformer := newSSETransformer(model)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var dataLines [][]byte
	var responseID, streamModel, role, finishReason string
	var created int64
	var contentBuilder bytes.Buffer
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
				ID      string `json:"id"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Role    string `json:"role"`
						Content string `json:"content"`
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
			for _, ch := range chunk.Choices {
				if ch.Delta.Role != "" && role == "" {
					role = ch.Delta.Role
				}
				if ch.Delta.Content != "" {
					contentBuilder.WriteString(ch.Delta.Content)
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
	resp := bufferedChatCompletion{
		ID: responseID, Object: "chat.completion", Created: created, Model: model,
		Choices: []struct {
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}{{
			Index: 0, FinishReason: finishReason,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: role, Content: contentBuilder.String()},
		}},
	}
	return json.Marshal(resp)
}
