package tools

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
)

func orchestrateRunFailure(err error) map[string]any {
	info := compile.DescribeFailure(err)
	kind := info.Kind
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		kind = "protocol"
	} else if kind == "environment" {
		kind = "runtime"
	}
	return map[string]any{
		"ok": false, "status": "error", "phase": "execution",
		"error_kind": kind, "error_type": kind + "_error", "error": err.Error(),
		"retryable": false, "retry_action": "inspect_execution",
	}
}
