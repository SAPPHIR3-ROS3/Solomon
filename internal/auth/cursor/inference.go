package cursor

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type inferenceParameter struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type inferenceModel struct {
	ID         string
	Display    string
	Parameters []inferenceParameter
}

// Resolve legacy picker IDs using Cursor's own mapping, rather than guessing
// suffixes or sending the legacy slug to the current inference endpoint.
func resolveInferenceModel(ctx context.Context, token, model string) (inferenceModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, modelsEndpoint, strings.NewReader(`{"useModelParameters":true}`))
	if err != nil {
		return inferenceModel{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("x-cursor-client-version", cursorClientVersionHeader())
	req.Header.Set("x-cursor-client-commit", cursorClientCommitHeader())
	req.Header.Set("x-cursor-client-type", "ide")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return inferenceModel{}, fmt.Errorf("cursor model catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return inferenceModel{}, fmt.Errorf("cursor model catalog: status %d", resp.StatusCode)
	}
	var catalog struct {
		Models []struct {
			Name     string `json:"name"`
			Variants []struct {
				LegacySlug     string               `json:"legacySlug"`
				Representation string               `json:"variantStringRepresentation"`
				Parameters     []inferenceParameter `json:"parameterValues"`
				Default        bool                 `json:"isDefaultNonMaxConfig"`
			} `json:"variants"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&catalog); err != nil {
		return inferenceModel{}, fmt.Errorf("cursor model catalog: %w", err)
	}
	model = normalizeChatModel(model)
	for _, entry := range catalog.Models {
		for _, variant := range entry.Variants {
			want := CanonicalCursorModelID(model)
			if variant.LegacySlug == model || variant.Representation == model || variant.LegacySlug == want || CanonicalCursorModelID(variant.LegacySlug) == want {
				return inferenceModel{ID: entry.Name, Display: firstNonEmpty(variant.LegacySlug, CanonicalCursorModelID(entry.Name)), Parameters: variant.Parameters}, nil
			}
		}
		if entry.Name == model || entry.Name == stripCursorModelPrefix(model) || CanonicalCursorModelID(entry.Name) == CanonicalCursorModelID(cursorBaseModelID(model)) {
			for _, variant := range entry.Variants {
				if variant.Default {
					return inferenceModel{ID: entry.Name, Display: firstNonEmpty(variant.LegacySlug, CanonicalCursorModelID(entry.Name)), Parameters: variant.Parameters}, nil
				}
			}
			return inferenceModel{ID: entry.Name, Display: CanonicalCursorModelID(entry.Name)}, nil
		}
	}
	return inferenceModel{}, fmt.Errorf("cursor chat: model %q is not in the current Cursor catalog", model)
}

func encodeInferenceRequest(model inferenceModel, turns []ChatTurn, tools []ChatTool) []byte {
	var request []byte
	for _, turn := range turns {
		var role uint64
		switch strings.ToLower(turn.Role) {
		case "user":
			role = 1
		case "assistant":
			role = 2
		case "system":
			role = 4
		case "tool":
			role = 3
		default:
			continue
		}
		message := pbVarint(1, role)
		if turn.Content != "" && role != 3 {
			message = append(message, pbString(2, turn.Content)...)
		}
		for _, call := range turn.ToolCalls {
			message = append(message, pbBytes(4, encodeInferenceToolCall(call))...)
		}
		if role == 3 {
			message = append(message, pbBytes(6, encodeInferenceToolResult(turn))...)
		}
		if len(message) == 0 {
			continue
		}
		request = append(request, pbBytes(1, message)...)
	}
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		definition := pbString(1, tool.Name)
		definition = append(definition, pbString(2, tool.Description)...)
		definition = append(definition, pbBytes(3, encodeInferenceStruct(tool.Parameters))...)
		request = append(request, pbBytes(2, definition)...)
	}
	selection := pbString(1, model.ID)
	for _, p := range model.Parameters {
		selection = append(selection, pbBytes(3, append(pbString(1, p.ID), pbString(2, p.Value)...))...)
	}
	selection = append(selection, pbVarint(4, 1)...)
	request = append(request, pbString(6, newCursorID())...)
	request = append(request, pbBytes(7, selection)...)
	return append(request, pbString(8, newCursorID())...)
}

func encodeInferenceToolCall(call ChatToolCall) []byte {
	out := pbString(1, call.ID)
	out = append(out, pbString(2, call.Name)...)
	args := strings.TrimSpace(call.Arguments)
	if args == "" {
		return out
	}
	var decoded any
	if json.Unmarshal([]byte(args), &decoded) == nil {
		if object, ok := decoded.(map[string]any); ok {
			out = append(out, pbBytes(3, encodeInferenceStruct(object))...)
			return out
		}
	}
	return append(out, pbString(4, args)...)
}

func encodeInferenceToolResult(turn ChatTurn) []byte {
	part := pbString(1, turn.ToolCallID)
	part = append(part, pbString(2, turn.ToolName)...)
	var decoded any
	if json.Unmarshal([]byte(strings.TrimSpace(turn.Content)), &decoded) == nil {
		part = append(part, pbBytes(3, encodeInferenceValue(decoded))...)
	} else {
		part = append(part, pbBytes(3, encodeInferenceValue(turn.Content))...)
	}
	if turn.ToolError {
		part = append(part, pbVarint(4, 1)...)
	}
	return pbBytes(1, part)
}

// Cursor uses google.protobuf.Struct/Value for tool schemas and arguments.
// Keeping this encoder here avoids introducing a generated protobuf dependency
// just for the small JSON value subset used by tool definitions.
func encodeInferenceStruct(values map[string]any) []byte {
	var out []byte
	for key, value := range values {
		entry := pbString(1, key)
		entry = append(entry, pbBytes(2, encodeInferenceValue(value))...)
		out = append(out, pbBytes(1, entry)...)
	}
	return out
}

func encodeInferenceValue(value any) []byte {
	switch typed := value.(type) {
	case nil:
		return pbVarint(1, 0)
	case bool:
		if typed {
			return pbVarint(4, 1)
		}
		return pbVarint(4, 0)
	case string:
		return pbString(3, typed)
	case float64:
		return pbFixed64(2, typed)
	case float32:
		return pbFixed64(2, float64(typed))
	case int:
		return pbFixed64(2, float64(typed))
	case int64:
		return pbFixed64(2, float64(typed))
	case []any:
		var list []byte
		for _, item := range typed {
			list = append(list, pbBytes(1, encodeInferenceValue(item))...)
		}
		return pbBytes(6, list)
	case map[string]any:
		return pbBytes(5, encodeInferenceStruct(typed))
	default:
		return pbString(3, fmt.Sprint(typed))
	}
}

func pbFixed64(field int, value float64) []byte {
	key := uvarint(uint64(field<<3 | 1))
	bits := math.Float64bits(value)
	var raw [8]byte
	binary.LittleEndian.PutUint64(raw[:], bits)
	return append(key, raw[:]...)
}

type inferenceField struct {
	number uint64
	data   []byte
}

func inferenceFields(data []byte) ([]inferenceField, error) {
	var fields []inferenceField
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 {
			return nil, fmt.Errorf("invalid protobuf tag")
		}
		data = data[n:]
		var value []byte
		switch tag & 7 {
		case 0:
			_, n = binary.Uvarint(data)
			if n <= 0 {
				return nil, fmt.Errorf("invalid protobuf varint")
			}
			value = data[:n]
			data = data[n:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				return nil, fmt.Errorf("invalid protobuf length")
			}
			data = data[n:]
			value = data[:size]
			data = data[size:]
		case 1:
			if len(data) < 8 {
				return nil, io.ErrUnexpectedEOF
			}
			data = data[8:]
		case 5:
			if len(data) < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			data = data[4:]
		default:
			return nil, fmt.Errorf("invalid protobuf wire type")
		}
		fields = append(fields, inferenceField{tag >> 3, value})
	}
	return fields, nil
}

func inferenceString(data []byte, number uint64) (string, error) {
	fields, err := inferenceFields(data)
	if err != nil {
		return "", err
	}
	for _, field := range fields {
		if field.number == number {
			return string(field.data), nil
		}
	}
	return "", nil
}

// Decode text parts and structured tool-call parts. Recursively extracting
// arbitrary protobuf strings would also emit reasoning, request IDs and final
// response metadata as chat text.
func readInferenceResponse(reader io.Reader, out io.Writer) (ChatResult, error) {
	var result ChatResult
	var text strings.Builder
	type toolAccumulator struct {
		id       string
		name     string
		args     strings.Builder
		complete bool
	}
	tools := map[int]*toolAccumulator{}
	nextToolIndex := 0
	for {
		var header [5]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
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
			if len(payload) > 4<<20 {
				return result, fmt.Errorf("cursor chat: decompressed frame too large")
			}
		}
		if header[0]&2 != 0 {
			if !json.Valid(payload) {
				return result, fmt.Errorf("cursor chat: invalid stream trailer")
			}
			if message := jsonChatError(payload); message != "" {
				return result, fmt.Errorf("cursor chat: %s", message)
			}
			result.Content = text.String()
			for index := 0; index <= nextToolIndex; index++ {
				if call, ok := tools[index]; ok && (call.complete || call.args.Len() > 0 || call.name != "") {
					result.ToolCalls = append(result.ToolCalls, ChatToolCall{ID: call.id, Name: call.name, Arguments: call.args.String()})
				}
			}
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
			return result, fmt.Errorf("cursor chat: %w", err)
		}
		for _, field := range fields {
			switch field.number {
			case 1:
				delta, err := inferenceString(field.data, 1)
				if err != nil {
					return result, err
				}
				text.WriteString(delta)
				if out != nil {
					if _, err := io.WriteString(out, delta); err != nil {
						return result, err
					}
				}
			case 2:
				part, err := inferenceFields(field.data)
				if err != nil {
					return result, fmt.Errorf("cursor chat: invalid tool call: %w", err)
				}
				index := nextToolIndex
				if rawIndex := inferenceVarint(part, 5); rawIndex >= 0 {
					index = int(rawIndex)
				} else {
					for existingIndex, existing := range tools {
						if existing.id != "" && existing.id == inferenceFieldString(part, 1) {
							index = existingIndex
							break
						}
					}
				}
				call := tools[index]
				if call == nil {
					call = &toolAccumulator{}
					tools[index] = call
				}
				if id := inferenceFieldString(part, 1); id != "" {
					call.id = id
				}
				if name := inferenceFieldString(part, 2); name != "" {
					call.name = name
				}
				if args := inferenceFieldString(part, 3); args != "" {
					call.args.WriteString(args)
				}
				if complete := inferenceVarint(part, 4); complete > 0 {
					call.complete = true
				}
				if index >= nextToolIndex {
					nextToolIndex = index + 1
				}
			case 8:
				errorFields, err := inferenceFields(field.data)
				if err != nil {
					return result, err
				}
				message := inferenceFieldString(errorFields, 1)
				code := inferenceFieldString(errorFields, 2)
				errorType := inferenceVarint(errorFields, 5)
				if code != "" || errorType >= 0 {
					return result, fmt.Errorf("cursor chat: %s (code=%s type=%d)", message, code, errorType)
				}
				return result, fmt.Errorf("cursor chat: %s", message)
			case 3:
				usage, err := inferenceFields(field.data)
				if err != nil {
					return result, err
				}
				if value := inferenceVarint(usage, 1); value >= 0 {
					result.Usage.PromptTokens = value
				}
				if value := inferenceVarint(usage, 2); value >= 0 {
					result.Usage.CompletionTokens = value
				}
				if value := inferenceVarint(usage, 3); value >= 0 {
					result.Usage.TotalTokens = value
				}
			}
		}
	}
}

func inferenceFieldString(fields []inferenceField, number uint64) string {
	for _, field := range fields {
		if field.number == number {
			return string(field.data)
		}
	}
	return ""
}

func inferenceVarint(fields []inferenceField, number uint64) int64 {
	for _, field := range fields {
		if field.number != number {
			continue
		}
		value, n := binary.Uvarint(field.data)
		if n > 0 {
			return int64(value)
		}
	}
	return -1
}
