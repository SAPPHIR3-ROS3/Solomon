package ipc

import "encoding/json"

type Envelope struct {
	Type string `json:"type"`
}

type Ping struct {
	Type string `json:"type"`
}

type Pong struct {
	Type string `json:"type"`
	OK   bool   `json:"ok"`
}

type RunRequest struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	WASM       []byte          `json:"wasm"`
	Mode       string          `json:"mode"`
	MaxCalls   int             `json:"max_calls"`
	TimeoutSec int             `json:"timeout_sec"`
}

type ToolRequest struct {
	Type   string          `json:"type"`
	RunID  string          `json:"run_id"`
	ReqID  string          `json:"req_id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
}

type ToolResponse struct {
	Type   string          `json:"type"`
	RunID  string          `json:"run_id"`
	ReqID  string          `json:"req_id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type RunDone struct {
	Type       string `json:"type"`
	RunID      string `json:"id"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	ToolCalls  int    `json:"tool_calls"`
	DurationMs int64  `json:"duration_ms"`
}

type Shutdown struct {
	Type string `json:"type"`
}

const (
	TypePing         = "ping"
	TypePong         = "pong"
	TypeRun          = "run"
	TypeToolRequest  = "tool_request"
	TypeToolResponse = "tool_response"
	TypeRunDone      = "run_done"
	TypeShutdown     = "shutdown"
)
