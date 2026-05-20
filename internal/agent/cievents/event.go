package cievents

import (
	"encoding/json"
	"time"
)

const SchemaVersion = 1

const (
	TypeRunStart       = "run_start"
	TypeAssistantStart = "assistant_start"
	TypeAssistantDelta = "assistant_delta"
	TypeAssistantEnd   = "assistant_end"
	TypeToolStart      = "tool_start"
	TypeToolResult     = "tool_result"
	TypeError          = "error"
	TypeRunEnd         = "run_end"
)

const (
	ChannelContent   = "content"
	ChannelReasoning = "reasoning"
)

type Event map[string]any

func baseEvent(typ string) Event {
	return Event{
		"v":    SchemaVersion,
		"type": typ,
		"ts":   time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func RunStart(prompt, model, provider, projHex string, ephemeral bool) Event {
	e := baseEvent(TypeRunStart)
	e["prompt"] = prompt
	e["model"] = model
	e["provider"] = provider
	e["proj_hex"] = projHex
	e["ephemeral"] = ephemeral
	return e
}

func AssistantStart(turn, checkpointSeq int) Event {
	e := baseEvent(TypeAssistantStart)
	e["turn"] = turn
	e["checkpoint_seq"] = checkpointSeq
	return e
}

func AssistantDelta(turn int, channel, delta string) Event {
	e := baseEvent(TypeAssistantDelta)
	e["turn"] = turn
	e["channel"] = channel
	e["delta"] = delta
	return e
}

func AssistantEnd(turn int, content, reasoning string, toolCalls []map[string]any) Event {
	e := baseEvent(TypeAssistantEnd)
	e["turn"] = turn
	e["content"] = content
	if reasoning != "" {
		e["reasoning"] = reasoning
	}
	if len(toolCalls) > 0 {
		e["tool_calls"] = toolCalls
	}
	return e
}

func ToolStart(turn int, id, name string, arguments json.RawMessage) Event {
	e := baseEvent(TypeToolStart)
	e["turn"] = turn
	e["id"] = id
	e["name"] = name
	var args any
	if len(arguments) > 0 {
		_ = json.Unmarshal(arguments, &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	e["arguments"] = args
	return e
}
