package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var errRPC = errors.New("sandbox rpc failed")

var errRPCResponseTooLarge = errors.New("sandbox rpc response too large; narrow the tool request or use pagination")

type rpcRequest struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

type rpcResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func marshalRPCRequest(name string, args []byte) ([]byte, error) {
	return json.Marshal(rpcRequest{Tool: name, Args: json.RawMessage(args)})
}

func decodeRPCResult(respBytes []byte) ([]byte, error) {
	var resp rpcResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}
		return nil, errRPC
	}
	return resp.Result, nil
}

func callTool(name string, args any) (json.RawMessage, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	if err := validateToolIntent(raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out, err := callToolRaw(name, raw)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

func validateToolIntent(raw json.RawMessage) error {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil || args == nil {
		return errors.New("tool call requires a non-empty intent string")
	}
	var intent string
	if err := json.Unmarshal(args["intent"], &intent); err != nil || strings.TrimSpace(intent) == "" {
		return errors.New("tool call requires a non-empty intent string")
	}
	return nil
}
