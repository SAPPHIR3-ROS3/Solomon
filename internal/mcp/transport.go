package mcp

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
)

type commandTransport struct {
	command string
	args    []string
	cwd     string
	env     map[string]string
	stderr  io.Writer
}

func (t *commandTransport) Connect(ctx context.Context) (sdkmcp.Connection, error) {
	cmd := exec.Command(t.command, t.args...)
	if t.cwd != "" {
		cmd.Dir = t.cwd
	}
	cmd.Env = os.Environ()
	for k, v := range t.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio stdin pipe failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio stdout pipe failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	if t.stderr != nil {
		cmd.Stderr = t.stderr
	}
	if err := cmd.Start(); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio process start failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	conn, err := (&sdkmcp.IOTransport{Reader: stdout, Writer: stdin}).Connect(ctx)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio IO transport connect failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waitDone)
	}()
	return &processConnection{Connection: conn, cmd: cmd, waitDone: waitDone}, nil
}

type processConnection struct {
	sdkmcp.Connection
	cmd      *exec.Cmd
	waitDone chan struct{}
}

func (c *processConnection) Close() error {
	err := c.Connection.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	<-c.waitDone
	return err
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for k, v := range t.headers {
		clone.Header.Set(k, v)
	}
	return t.base.RoundTrip(clone)
}

func httpClientWithHeaders(headers map[string]string) *http.Client {
	base := http.DefaultTransport
	if len(headers) == 0 {
		return http.DefaultClient
	}
	return &http.Client{Transport: &headerTransport{base: base, headers: headers}}
}
