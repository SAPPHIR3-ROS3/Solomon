package cursor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// This is the small wire subset Solomon uses from Cursor Agent v1. Field
// numbers follow the MIT-licensed agent.proto revision cited in
// agentproto/NOTICE. Unknown fields are skipped so Cursor can add unrelated
// message variants.
type agentWireField struct {
	number int
	wire   int
	bytes  []byte
	value  uint64
}

func agentWireFields(data []byte) ([]agentWireField, error) {
	var fields []agentWireField
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("invalid protobuf tag")
		}
		data = data[n:]
		number, wire := int(tag>>3), int(tag&7)
		if number <= 0 {
			return nil, fmt.Errorf("invalid protobuf field number %d", number)
		}
		field := agentWireField{number: number, wire: wire}
		switch wire {
		case 0:
			value, consumed := binary.Uvarint(data)
			if consumed <= 0 {
				return nil, fmt.Errorf("invalid protobuf varint in field %d", number)
			}
			field.value = value
			data = data[consumed:]
		case 1:
			if len(data) < 8 {
				return nil, fmt.Errorf("truncated fixed64 field %d", number)
			}
			data = data[8:]
		case 2:
			size, consumed := binary.Uvarint(data)
			if consumed <= 0 || size > uint64(len(data)-consumed) {
				return nil, fmt.Errorf("invalid length-delimited field %d", number)
			}
			start := consumed
			field.bytes = data[start : start+int(size)]
			data = data[start+int(size):]
		case 5:
			if len(data) < 4 {
				return nil, fmt.Errorf("truncated fixed32 field %d", number)
			}
			data = data[4:]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d in field %d", wire, number)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func lastAgentWireField(fields []agentWireField, matches func(int) bool) (agentWireField, bool) {
	for i := len(fields) - 1; i >= 0; i-- {
		if matches(fields[i].number) {
			return fields[i], true
		}
	}
	return agentWireField{}, false
}

func agentWireVarint(fields []agentWireField, number int) uint64 {
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].number == number && fields[i].wire == 0 {
			return fields[i].value
		}
	}
	return 0
}

func agentWireBytes(fields []agentWireField, number int) []byte {
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].number == number && fields[i].wire == 2 {
			return fields[i].bytes
		}
	}
	return nil
}

func DecodeAgentServerMessage(data []byte) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent protocol decode: %w", err)}
	}
	message, ok := lastAgentWireField(fields, func(int) bool { return true })
	if !ok {
		return AgentEvent{Unknown: "server:empty"}
	}
	if message.wire != 2 {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent envelope field %d is not length-delimited", message.number)}
	}
	switch message.number {
	case 1:
		return decodeAgentInteraction(message.bytes)
	case 2:
		return decodeAgentExec(message.bytes)
	case 3:
		return AgentEvent{Unknown: "checkpoint"}
	case 4:
		return decodeAgentKV(message.bytes)
	case 7:
		return decodeAgentQuery(message.bytes)
	default:
		return AgentEvent{Unknown: fmt.Sprintf("server:field_%d", message.number)}
	}
}

func decodeAgentInteraction(data []byte) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent interaction decode: %w", err)}
	}
	update, ok := lastAgentWireField(fields, func(int) bool { return true })
	if !ok {
		return AgentEvent{Unknown: "interaction:empty"}
	}
	switch update.number {
	case 1, 4:
		payload, err := agentWireFields(update.bytes)
		if err != nil {
			return AgentEvent{Err: fmt.Errorf("Cursor Agent text update decode: %w", err)}
		}
		text := string(agentWireBytes(payload, 1))
		if update.number == 1 {
			return AgentEvent{Text: text}
		}
		return AgentEvent{Thinking: text}
	case 14:
		usageFields, err := agentWireFields(update.bytes)
		if err != nil {
			return AgentEvent{Err: fmt.Errorf("Cursor Agent usage decode: %w", err)}
		}
		input := int64(agentWireVarint(usageFields, 1))
		output := int64(agentWireVarint(usageFields, 2))
		return AgentEvent{Ended: true, Usage: ChatUsage{
			PromptTokens: input, CompletionTokens: output, TotalTokens: input + output,
			CachedPromptTokens: int64(agentWireVarint(usageFields, 3)),
			CacheWriteTokens:   int64(agentWireVarint(usageFields, 4)),
			ReasoningTokens:    int64(agentWireVarint(usageFields, 5)),
		}}
	default:
		return AgentEvent{Unknown: fmt.Sprintf("interaction:field_%d", update.number)}
	}
}

// AssistantTextFromMessage extracts displayable text from a Cursor KV message
// blob. Reasoning and non-text parts are intentionally excluded.
func AssistantTextFromMessage(data []byte) (string, bool) {
	text, isAssistant := assistantMessageText(data)
	return text, isAssistant && text != ""
}

func assistantMessageText(data []byte) (string, bool) {
	var message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(data, &message) != nil || message.Role != "assistant" {
		return "", false
	}
	var content string
	if json.Unmarshal(message.Content, &content) == nil {
		return content, true
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(message.Content, &parts) != nil {
		return "", true
	}
	var text strings.Builder
	for _, part := range parts {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	return text.String(), true
}

func decodeAgentQuery(data []byte) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent query decode: %w", err)}
	}
	query, ok := lastAgentWireField(fields, func(number int) bool { return number != 1 })
	if !ok {
		return AgentEvent{Unknown: "query:empty"}
	}
	kind := 0
	switch query.number {
	case 2, 9:
		kind = query.number
	case 3, 4, 5, 6, 7:
		kind = query.number
	}
	return AgentEvent{Query: &AgentQueryRequest{ID: uint32(agentWireVarint(fields, 1)), Kind: kind}}
}

func decodeAgentKV(data []byte) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent KV decode: %w", err)}
	}
	request, ok := lastAgentWireField(fields, func(number int) bool { return number == 2 || number == 3 })
	if !ok {
		return AgentEvent{Unknown: "kv:empty"}
	}
	args, err := agentWireFields(request.bytes)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent KV args decode: %w", err)}
	}
	if request.number == 3 {
		return AgentEvent{KV: &AgentKVRequest{ID: uint32(agentWireVarint(fields, 1)), BlobID: append([]byte(nil), agentWireBytes(args, 1)...), BlobData: append([]byte(nil), agentWireBytes(args, 2)...), Set: true}}
	}
	return AgentEvent{KV: &AgentKVRequest{ID: uint32(agentWireVarint(fields, 1)), BlobID: append([]byte(nil), agentWireBytes(args, 1)...)}}
}

func decodeAgentExec(data []byte) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent exec decode: %w", err)}
	}
	message, ok := lastAgentWireField(fields, func(number int) bool {
		return number != 1 && number != 15 && number != 19 && number != 55
	})
	if !ok {
		return AgentEvent{Unknown: "exec:empty"}
	}
	base := AgentExecRequest{ID: uint32(agentWireVarint(fields, 1)), ExecID: string(agentWireBytes(fields, 15))}
	switch message.number {
	case 10:
		return AgentEvent{Context: &base}
	case 36:
		return AgentEvent{MCPState: &base}
	case 11:
		return decodeAgentTool(message.bytes, base)
	}
	request := &AgentNativeRequest{AgentExecRequest: base}
	switch message.number {
	case 2:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "shell", 2, 4, 3
	case 14:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "shell stream", 14, 5, 3
	case 16:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "background shell", 16, 3, 3
	case 7, 29:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "read", 7, 3, 2
	case 3:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "write", 3, 6, 2
	case 4:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "delete", 4, 6, 2
	case 8:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "list", 8, 3, 2
	case 9:
		request.Kind, request.ResultField, request.RejectedField, request.ReasonField = "diagnostics", 9, 3, 2
	case 5:
		request.Kind, request.ResultField = "grep", 5
	case 20:
		request.Kind, request.ResultField = "fetch", 20
	case 41:
		request.Kind = "precheck_shell"
	case 42:
		request.Kind = "precheck_mcp"
	case 43:
		request.Kind = "precheck_web_fetch"
	default:
		return AgentEvent{Unknown: fmt.Sprintf("exec:field_%d", message.number)}
	}
	return AgentEvent{Native: request}
}

func decodeAgentTool(data []byte, base AgentExecRequest) AgentEvent {
	fields, err := agentWireFields(data)
	if err != nil {
		return AgentEvent{Err: fmt.Errorf("Cursor Agent tool decode: %w", err)}
	}
	arguments := make(map[string]any)
	for _, entry := range fields {
		if entry.number != 2 || entry.wire != 2 {
			continue
		}
		pair, err := agentWireFields(entry.bytes)
		if err != nil {
			return AgentEvent{Err: fmt.Errorf("Cursor Agent tool arguments decode: %w", err)}
		}
		name := string(agentWireBytes(pair, 1))
		raw := agentWireBytes(pair, 2)
		if name == "" {
			continue
		}
		var value structpb.Value
		if err := proto.Unmarshal(raw, &value); err == nil {
			arguments[name] = value.AsInterface()
		} else {
			arguments[name] = string(raw)
		}
	}
	encoded, _ := json.Marshal(arguments)
	return AgentEvent{Tool: &AgentToolCall{
		ID: string(agentWireBytes(fields, 3)), Name: firstNonEmpty(string(agentWireBytes(fields, 5)), string(agentWireBytes(fields, 1))),
		Arguments: string(encoded), ExecID: base.ExecID, ExecNumber: base.ID,
	}}
}
