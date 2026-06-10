package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/ipc"
)

type IPCBridge struct {
	in    *bufio.Reader
	out   io.Writer
	runID string
}

func (b *IPCBridge) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	msg := ipc.ToolRequest{
		Type:  ipc.TypeToolRequest,
		RunID: b.runID,
		ReqID: reqID,
		Name:  name,
		Args:  args,
	}
	if err := writeJSON(b.out, msg); err != nil {
		return nil, err
	}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		line, err := b.in.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var env ipc.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			return nil, err
		}
		if env.Type != ipc.TypeToolResponse {
			return nil, fmt.Errorf("expected tool_response, got %q", env.Type)
		}
		var resp ipc.ToolResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, err
		}
		if resp.RunID != b.runID || resp.ReqID != reqID {
			continue
		}
		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		return resp.Result, nil
	}
}
