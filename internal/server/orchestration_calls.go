package server

import "encoding/json"

func orchestrateAPICalls(parent apiToolCall) []apiToolCall {
	raw, err := json.Marshal(parent.Result["sdk_tool_calls"])
	if err != nil {
		return nil
	}
	var calls []struct {
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Result    json.RawMessage `json:"result"`
	}
	if json.Unmarshal(raw, &calls) != nil {
		return nil
	}
	out := make([]apiToolCall, 0, len(calls))
	for _, call := range calls {
		args := decodeArguments(string(call.Arguments))
		result := apiToolResult(string(call.Result))
		status := stringField(result, "status")
		out = append(out, apiToolCall{
			ID: call.ID, Name: call.Name, Input: toolInput(call.Name, args, string(call.Arguments)),
			Intent: stringField(args, "intent"), Parameters: toolParameters(call.Name, args),
			Status: status, Result: result, CheckpointSeq: parent.CheckpointSeq, CheckpointBranch: parent.CheckpointBranch,
		})
	}
	return out
}
