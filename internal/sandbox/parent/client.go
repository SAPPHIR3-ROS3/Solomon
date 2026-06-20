package parent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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
	_ = ctx
	exe, err := os.Executable()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox worker executable resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	cmd := exec.Command(exe, "sandbox-worker")
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
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox worker start failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	c := &Client{cmd: cmd, in: stdin, out: bufio.NewReader(stdout)}
	if err := c.ping(); err != nil {
		c.Close()
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox worker ping failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	return c, nil
}

func (c *Client) ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("worker closed")
	}
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
		c.noteIPCDead(err)
		return ipc.RunDone{}, err
	}
	for {
		line, err := c.out.ReadBytes('\n')
		if err != nil {
			c.noteIPCDead(err)
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker ipc read failed", logging.LogOptions{Params: map[string]any{"run_id": id, "err": err.Error()}})
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
				c.noteIPCDead(err)
				return ipc.RunDone{}, err
			}
		case ipc.TypeRunDone:
			var done ipc.RunDone
			if err := json.Unmarshal(line, &done); err != nil {
				return ipc.RunDone{}, err
			}
			return done, nil
		default:
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker unexpected ipc message", logging.LogOptions{Params: map[string]any{"type": env.Type, "run_id": id}})
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
		if pingErr := globalClient.ping(); pingErr == nil {
			return globalClient, nil
		} else {
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker reconnecting after ping failure", logging.LogOptions{Params: map[string]any{"err": pingErr.Error()}})
		}
		_ = globalClient.Close()
		globalClient = nil
	}
	c, err := Start(ctx)
	if err != nil {
		return nil, err
	}
	globalClient = c
	return c, nil
}

func RunGlobal(ctx context.Context, wasm []byte, mode string, exec ToolExec) (ipc.RunDone, error) {
	done, err := runOnce(ctx, wasm, mode, exec)
	if err == nil || !isIPCDead(err) {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox run failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		} else if strings.TrimSpace(done.Error) != "" {
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox script run error", logging.LogOptions{Params: map[string]any{"error": done.Error, "tool_calls": done.ToolCalls}})
		}
		return done, err
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker ipc dead; retrying run", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	CloseGlobal()
	return runOnce(ctx, wasm, mode, exec)
}

func runOnce(ctx context.Context, wasm []byte, mode string, exec ToolExec) (ipc.RunDone, error) {
	client, err := Global(ctx)
	if err != nil {
		return ipc.RunDone{}, err
	}
	return client.Run(ctx, wasm, mode, exec)
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

func SimulateWorkerCrash() {
	globalMu.Lock()
	c := globalClient
	globalMu.Unlock()
	if c == nil {
		return
	}
	c.forceKill()
}

func (c *Client) noteIPCDead(err error) {
	if !isIPCDead(err) {
		return
	}
	c.markBroken()
	forgetGlobal(c)
}

func (c *Client) markBroken() {
	if c.closed {
		return
	}
	c.closed = true
	if c.in != nil {
		_ = c.in.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
}

func (c *Client) forceKill() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markBroken()
	forgetGlobal(c)
}

func forgetGlobal(c *Client) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalClient == c {
		globalClient = nil
	}
}

func isIPCDead(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "file already closed")
}
