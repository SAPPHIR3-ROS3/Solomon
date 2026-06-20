package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/host"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/ipc"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
)

func Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	br := bufio.NewReader(in)
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "sandbox worker ipc read failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return err
		}
		var env ipc.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			return err
		}
		switch env.Type {
		case ipc.TypePing:
			if err := writeJSON(out, ipc.Pong{Type: ipc.TypePong, OK: true}); err != nil {
				return err
			}
		case ipc.TypeShutdown:
			return nil
		case ipc.TypeRun:
			var req ipc.RunRequest
			if err := json.Unmarshal(line, &req); err != nil {
				return err
			}
			if err := handleRun(ctx, br, out, req); err != nil {
				return err
			}
		default:
			logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker unknown ipc type", logging.LogOptions{Params: map[string]any{"type": env.Type}})
			return fmt.Errorf("unknown ipc type %q", env.Type)
		}
	}
}

func handleRun(ctx context.Context, br *bufio.Reader, out io.Writer, req ipc.RunRequest) error {
	start := time.Now()
	bridge := &IPCBridge{in: br, out: out, runID: req.ID}
	caller := &host.CountingCaller{Inner: bridge, MaxCalls: req.MaxCalls}
	if caller.MaxCalls <= 0 {
		caller.MaxCalls = 256
	}
	timeout := time.Duration(req.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rn, err := run.NewRunner(runCtx, caller, run.Config{MaxToolCalls: caller.MaxCalls, Timeout: timeout})
	if err != nil {
		return writeJSON(out, ipc.RunDone{Type: ipc.TypeRunDone, RunID: req.ID, Error: err.Error()})
	}
	defer rn.Close(runCtx)

	output, runErr := rn.Run(runCtx, req.WASM)
	done := ipc.RunDone{
		Type:       ipc.TypeRunDone,
		RunID:      req.ID,
		Output:     output,
		DurationMs: time.Since(start).Milliseconds(),
		ToolCalls:  caller.ToolCalls(),
	}
	if runErr != nil {
		done.Error = runErr.Error()
		logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker script run failed", logging.LogOptions{Params: map[string]any{"run_id": req.ID, "err": runErr.Error()}})
	} else if caller.LastError != nil {
		done.Error = caller.LastError.Error()
		logging.Log(logging.WARNING_LOG_LEVEL, "sandbox worker tool call failed", logging.LogOptions{Params: map[string]any{"run_id": req.ID, "err": caller.LastError.Error()}})
	}
	return writeJSON(out, done)
}

func writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

func Main() {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	ctx := context.Background()
	if err := Serve(ctx, os.Stdin, os.Stdout); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "sandbox worker exited with error", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		fmt.Fprintf(os.Stderr, "sandbox worker: %v\n", err)
		os.Exit(1)
	}
}
