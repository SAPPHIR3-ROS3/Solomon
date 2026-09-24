package cursor

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// encodeUnifiedChatRequest is the ChatService StreamUnifiedChat body.
func encodeUnifiedChatRequest(model string, turns []ChatTurn, tools []ChatTool) []byte {
	model = normalizeChatModel(model)
	var request []byte
	for _, turn := range turns {
		role := strings.ToLower(strings.TrimSpace(turn.Role))
		switch role {
		case "system":
			continue
		case "assistant":
			msg := pbVarint(2, 2)
			if turn.Content != "" {
				msg = append(pbString(1, turn.Content), msg...)
			}
			for _, call := range turn.ToolCalls {
				result := pbString(1, call.ID)
				result = append(result, pbString(2, call.Name)...)
				result = append(result, pbString(4, call.Arguments)...)
				msg = append(msg, pbBytes(18, result)...)
			}
			if len(msg) > 0 {
				request = append(request, pbBytes(1, msg)...)
			}
		case "tool":
			result := pbString(1, turn.ToolCallID)
			result = append(result, pbString(2, turn.ToolName)...)
			result = append(result, pbString(7, turn.Content)...)
			msg := pbVarint(2, 1)
			msg = append(msg, pbBytes(18, result)...)
			request = append(request, pbBytes(1, msg)...)
		default:
			if strings.TrimSpace(turn.Content) == "" {
				continue
			}
			msg := pbString(1, turn.Content)
			msg = append(msg, pbVarint(2, 1)...)
			msg = append(msg, pbString(13, newCursorID())...)
			request = append(request, pbBytes(1, msg)...)
		}
	}
	details := pbString(1, CanonicalCursorModelID(model))
	request = append(request, pbBytes(5, details)...)
	request = append(request, pbVarint(22, 1)...)
	request = append(request, pbString(23, newCursorID())...)
	env := pbString(1, cursorClientOS())
	env = append(env, pbString(2, cursorClientArch())...)
	env = append(env, pbString(3, cursorClientOSVersion())...)
	env = append(env, pbString(4, runtime.GOOS)...)
	env = append(env, pbString(5, time.Now().Format(time.RFC3339Nano))...)
	env = append(env, pbString(7, cursorClientVersionHeader())...)
	env = append(env, pbString(9, cursorClientOS())...)
	request = append(request, pbBytes(26, env)...)
	request = append(request, pbVarint(45, 1)...)
	request = append(request, pbVarint(69, 1)...)
	if len(tools) > 0 {
		request = append(request, pbVarint(27, 1)...)
		request = append(request, pbVarint(29, 19)...)
		request = append(request, pbVarint(29, 49)...)
	}
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		params, _ := json.Marshal(tool.Parameters)
		if len(params) == 0 {
			params = []byte("{}")
		}
		def := pbString(1, tool.Name)
		def = append(def, pbString(2, tool.Description)...)
		def = append(def, pbString(3, string(params))...)
		def = append(def, pbString(4, "solomon")...)
		request = append(request, pbBytes(34, def)...)
	}
	return request
}

func encodeLegacyChatRequest(model string, turns []ChatTurn) []byte {
	return encodeUnifiedChatRequest(model, turns, nil)
}

func readUnifiedChatResponse(reader io.Reader, out io.Writer) (ChatResult, error) {
	var result ChatResult
	var text strings.Builder
	for {
		var header [5]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			if text.Len() > 0 {
				result.Content = text.String()
				result.FinishReason = "stop"
				return result, nil
			}
			return result, fmt.Errorf("cursor chat: incomplete stream: %w", err)
		}
		size := binary.BigEndian.Uint32(header[1:])
		if size > 4<<20 {
			return result, fmt.Errorf("cursor chat: response frame too large")
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return result, fmt.Errorf("cursor chat: incomplete frame: %w", err)
		}
		if header[0]&1 != 0 {
			zr, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				return result, fmt.Errorf("cursor chat: invalid gzip: %w", err)
			}
			payload, err = io.ReadAll(io.LimitReader(zr, (4<<20)+1))
			_ = zr.Close()
			if err != nil {
				return result, err
			}
		}
		if header[0]&2 != 0 {
			if message := jsonChatError(payload); message != "" {
				return result, fmt.Errorf("cursor chat: %s", message)
			}
			result.Content = text.String()
			if len(result.ToolCalls) > 0 {
				result.FinishReason = "tool_calls"
			} else if text.Len() > 0 {
				result.FinishReason = "stop"
			} else {
				return ChatResult{}, fmt.Errorf("cursor chat: empty response")
			}
			return result, nil
		}
		fields, err := inferenceFields(payload)
		if err != nil {
			continue
		}
		wrapped := false
		for _, field := range fields {
			if field.number == 2 {
				wrapped = true
				break
			}
		}
		if wrapped {
			for _, field := range fields {
				switch field.number {
				case 1:
					if call := parseUnifiedToolCall(field.data); call.Name != "" || call.ID != "" {
						result.ToolCalls = append(result.ToolCalls, call)
					}
				case 2:
					inner, err := inferenceFields(field.data)
					if err != nil {
						continue
					}
					if err := applyUnifiedResponseFields(&result, &text, out, inner); err != nil {
						return result, err
					}
				}
			}
			continue
		}
		if err := applyUnifiedResponseFields(&result, &text, out, fields); err != nil {
			return result, err
		}
	}
}

func applyUnifiedResponseFields(result *ChatResult, text *strings.Builder, out io.Writer, fields []inferenceField) error {
	for _, field := range fields {
		switch field.number {
		case 1:
			if len(field.data) == 0 {
				continue
			}
			text.Write(field.data)
			if out != nil {
				if _, err := out.Write(field.data); err != nil {
					return err
				}
			}
		case 13, 36:
			if call := parseUnifiedToolCall(field.data); call.Name != "" || call.ID != "" {
				result.ToolCalls = append(result.ToolCalls, call)
			}
		}
	}
	return nil
}

func parseUnifiedToolCall(data []byte) ChatToolCall {
	fields, err := inferenceFields(data)
	if err != nil {
		return ChatToolCall{}
	}
	call := ChatToolCall{}
	for _, field := range fields {
		switch field.number {
		case 2:
			call.ID = string(field.data)
		case 27:
			inner, err := inferenceFields(field.data)
			if err != nil {
				continue
			}
			for _, part := range inner {
				switch part.number {
				case 2:
					call.Name = string(part.data)
				case 3:
					if raw, err := json.Marshal(protoStructMap(part.data)); err == nil {
						call.Arguments = string(raw)
					}
				}
			}
		}
	}
	return call
}

func protoStructMap(data []byte) map[string]any {
	out := map[string]any{}
	fields, err := inferenceFields(data)
	if err != nil {
		return out
	}
	for _, field := range fields {
		if field.number != 1 {
			continue
		}
		entry, err := inferenceFields(field.data)
		if err != nil {
			continue
		}
		key := ""
		var value any
		for _, part := range entry {
			switch part.number {
			case 1:
				key = string(part.data)
			case 2:
				value = protoValue(part.data)
			}
		}
		if key != "" {
			out[key] = value
		}
	}
	return out
}

func protoValue(data []byte) any {
	fields, err := inferenceFields(data)
	if err != nil || len(fields) == 0 {
		return string(data)
	}
	field := fields[0]
	switch field.number {
	case 3:
		return string(field.data)
	case 5:
		return protoStructMap(field.data)
	default:
		return string(field.data)
	}
}

func readLegacyChatResponse(reader io.Reader, out io.Writer) (ChatResult, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, 4<<20))
	if err != nil {
		return ChatResult{}, err
	}
	raw = maybeGunzip(raw)
	if message := jsonChatError(raw); message != "" {
		return ChatResult{}, fmt.Errorf("cursor chat: %s", message)
	}
	text := parseLegacyChatText(raw)
	if text == "" {
		return ChatResult{}, io.ErrUnexpectedEOF
	}
	if out != nil {
		_, _ = io.WriteString(out, text)
	}
	return ChatResult{Content: text, FinishReason: "stop"}, nil
}

func parseLegacyChatText(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if text := legacyJSONChatText(raw); text != "" {
		return text
	}
	var full strings.Builder
	rest := drainLegacyChatFrames(append([]byte(nil), raw...), &full)
	if full.Len() == 0 && len(rest) > 0 {
		writeLegacyChatText(protoUTF8Strings(rest), &full)
	}
	return full.String()
}

func legacyJSONChatText(raw []byte) string {
	var payload any
	if jsonUnmarshal(raw, &payload) == nil {
		return findLegacyJSONText(payload)
	}
	var b strings.Builder
	for _, frame := range connectJSONPayloads(raw) {
		var item any
		if jsonUnmarshal([]byte(frame), &item) == nil {
			b.WriteString(findLegacyJSONText(item))
		}
	}
	return b.String()
}

func drainLegacyChatFrames(buf []byte, full *strings.Builder) []byte {
	for len(buf) >= 5 {
		n := int(binary.BigEndian.Uint32(buf[1:5]))
		if n < 0 || n > 1<<22 {
			writeLegacyChatText(protoUTF8Strings(buf), full)
			return nil
		}
		if len(buf) < 5+n {
			return buf
		}
		payload := buf[5 : 5+n]
		if text := legacyJSONChatText(payload); text != "" && !looksLikeLegacyRPCError(text) {
			full.WriteString(text)
		} else {
			writeLegacyChatText(protoUTF8Strings(payload), full)
		}
		buf = buf[5+n:]
	}
	return buf
}

func writeLegacyChatText(parts []string, full *strings.Builder) {
	for _, part := range parts {
		if part == "" || looksLikeLegacyMeta(part) || looksLikeLegacyRPCError(part) {
			continue
		}
		full.WriteString(part)
	}
}

func looksLikeLegacyRPCError(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return low == "error" || low == "failed" || low == "unimplemented" || strings.Contains(low, "streamunifiedchat") || strings.Contains(low, "invalid request")
}

func findLegacyJSONText(v any) string {
	switch typed := v.(type) {
	case map[string]any:
		if typed["error"] != nil {
			return findLegacyJSONText(typed["result"])
		}
		for _, key := range []string{"text", "content", "delta", "output"} {
			if s, ok := typed[key].(string); ok {
				s = strings.TrimSpace(s)
				if s != "" && !looksLikeLegacyMeta(s) && !looksLikeLegacyRPCError(s) {
					return s
				}
			}
		}
		var b strings.Builder
		for k, child := range typed {
			if k != "message" && k != "code" {
				b.WriteString(findLegacyJSONText(child))
			}
		}
		return b.String()
	case []any:
		var b strings.Builder
		for _, child := range typed {
			b.WriteString(findLegacyJSONText(child))
		}
		return b.String()
	default:
		return ""
	}
}

func jsonUnmarshal(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

func looksLikeLegacyMeta(s string) bool {
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(s))
	return low == "composer-2.5" || low == "grok-4.6" || low == "grok-4.5" || strings.HasPrefix(low, "cursor-")
}

func protoUTF8Strings(b []byte) []string {
	var out []string
	i := 0
	for i < len(b) {
		key, n := binary.Uvarint(b[i:])
		if n <= 0 {
			break
		}
		i += n
		switch key & 7 {
		case 0:
			_, n = binary.Uvarint(b[i:])
			if n <= 0 {
				return out
			}
			i += n
		case 1:
			if i+8 > len(b) {
				return out
			}
			i += 8
		case 5:
			if i+4 > len(b) {
				return out
			}
			i += 4
		case 2:
			ln, n := binary.Uvarint(b[i:])
			if n <= 0 {
				return out
			}
			i += n
			end := i + int(ln)
			if end > len(b) {
				return out
			}
			chunk := b[i:end]
			if utf8.Valid(chunk) {
				if text := strings.TrimSpace(string(chunk)); text != "" && !strings.ContainsRune(text, 0) {
					out = append(out, text)
				}
			}
			out = append(out, protoUTF8Strings(chunk)...)
			i = end
		default:
			return out
		}
	}
	return out
}
