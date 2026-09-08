package sdk

import (
	"encoding/json"
	"fmt"
)

// MCPCall is the lowered form of sdk.mcp.<tool>(intent, args). It is kept
// public only as the compiler target for the Code Mode MCP namespace.
func MCPCall(name, intent string, args any) (json.RawMessage, error) {
	var object map[string]any
	if args == nil {
		object = map[string]any{}
	} else {
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, fmt.Errorf("MCP tool arguments must be an object")
		}
	}
	object["intent"] = intent
	return callTool(name, object)
}
