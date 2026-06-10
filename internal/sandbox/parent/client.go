package parent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/ipc"
)

type ToolExec func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)

type Client struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	closed bool
}

func Start(ctx context.Context) (*Client, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, exe, "sandbox-worker")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}
	c := &Client{cmd: cmd, in: stdin, out: bufio.NewReader(stdout)}
	if err := c.ping(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) ping() error {
	if err := c.writeJSON(ipc.Ping{Type: ipc.TypePing}); err != nil {
		return err
	}
	line, err := c.out.ReadBytes('\n')
	if err != nil {
		return err
	}
	var pong ipc.Pong
	if err := json.Unmarshal(line, &pong); err != nil {
		return err
	}
	if !pong.OK {
		return fmt.Errorf("worker ping failed")
	}
	return nil
}

func (c *Client) Run(ctx context.Context, wasm []byte, mode string, exec ToolExec) (ipc.RunDone, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ipc.RunDone{}, fmt.Errorf("worker closed")
	}
	id := fmt.Sprintf("run-%d", time.Now().UnixNano())
	req := ipc.RunRequest{
		Type:       ipc.TypeRun,
		ID:         id,
		WASM:       wasm,
		Mode:       mode,
		MaxCalls:   256,
		TimeoutSec: 300,
	}
	if err := c.writeJSON(req); err != nil {
		return ipc.RunDone{}, err
	}
	for {
		line, err := c.out.ReadBytes('\n')
		if err != nil {
			return ipc.RunDone{}, err
		}
		var env ipc.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			return ipc.RunDone{}, err
		}
		switch env.Type {
		case ipc.TypeToolRequest:
			var tr ipc.ToolRequest
			if err := json.Unmarshal(line, &tr); err != nil {
				return ipc.RunDone{}, err
			}
			resp := ipc.ToolResponse{Type: ipc.TypeToolResponse, RunID: tr.RunID, ReqID: tr.ReqID}
			result, err := exec(ctx, tr.Name, tr.Args)
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
			if err := c.writeJSON(resp); err != nil {
				return ipc.RunDone{}, err
			}
		case ipc.TypeRunDone:
			var done ipc.RunDone
			if err := json.Unmarshal(line, &done); err != nil {
				return ipc.RunDone{}, err
			}
			return done, nil
		default:
			return ipc.RunDone{}, fmt.Errorf("unexpected ipc %q during run", env.Type)
		}
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	_ = c.writeJSON(ipc.Shutdown{Type: ipc.TypeShutdown})
	if c.in != nil {
		_ = c.in.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Wait()
	}
	return nil
}

func (c *Client) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.in, "%s\n", b)
	return err
}

var (
	globalMu     sync.Mutex
	globalClient *Client
)

func Global(ctx context.Context) (*Client, error) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalClient != nil {
		return globalClient, nil
	}
	c, err := Start(ctx)
	if err != nil {
		return nil, err
	}
	globalClient = c
	return c, nil
}

func Warm(ctx context.Context, version string) {
	_, _ = compile.EnsureReferenceWASM(version)
	_, _ = Global(ctx)
}

func CloseGlobal() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalClient != nil {
		_ = globalClient.Close()
		globalClient = nil
	}
}
