package cievents

import "encoding/json"

func ToolResult(turn int, id, name string, result json.RawMessage, errMsg string) Event {
	e := baseEvent(TypeToolResult)
	e["turn"] = turn
	e["id"] = id
	e["name"] = name
	var res any
	if len(result) > 0 {
		_ = json.Unmarshal(result, &res)
	}
	if res == nil {
		res = map[string]any{}
	}
	e["result"] = res
	if errMsg != "" {
		e["error"] = errMsg
	}
	return e
}

func ErrorEvent(code int, message string) Event {
	e := baseEvent(TypeError)
	e["code"] = code
	e["message"] = message
	return e
}

func RunEnd(exitCode int, exitReason, finalContent string, usage any) Event {
	e := baseEvent(TypeRunEnd)
	e["exit_code"] = exitCode
	e["exit_reason"] = exitReason
	if finalContent != "" {
		e["final_content"] = finalContent
	}
	if usage != nil {
		e["usage"] = usage
	}
	return e
}
