package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server/detach"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func Run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("missing server subcommand")
	}
	switch args[0] {
	case "start":
		mode, devDir, err := parseStart(args[1:])
		if err != nil {
			return err
		}
		return start(mode, devDir)
	case "run":
		mode, devDir, err := parseRun(args[1:])
		if err != nil {
			return err
		}
		return runProcess(mode, devDir)
	case "status":
		status()
	case "stop":
		if err := stop(); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("server: stopped")
			} else {
				return err
			}
		}
	case "restart":
		mode, devDir := "normal", ""
		if state, err := serverruntime.LoadRunningState(context.Background()); err == nil {
			mode, devDir = state.Mode, state.DevDir
		}
		if err := stop(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return start(mode, devDir)
	case "logs":
		interactive := len(args) == 2 && args[1] == "interactive"
		if len(args) > 2 || (len(args) == 2 && !interactive) {
			return fmt.Errorf("usage: solomon server logs [interactive]")
		}
		return logs(interactive)
	default:
		usage()
		return fmt.Errorf("unknown server subcommand: %s", args[0])
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: solomon server <start|status|stop|restart|logs>")
}

func parseStart(args []string) (string, string, error) {
	if len(args) == 0 {
		return "normal", "", nil
	}
	if len(args) != 2 || args[0] != "dev" {
		return "", "", fmt.Errorf("usage: solomon server start [dev <gui-directory>]")
	}
	directory, err := validateDevDirectory(args[1])
	if err != nil {
		return "", "", err
	}
	return "dev", directory, nil
}

func parseRun(args []string) (string, string, error) {
	if len(args) == 0 {
		return "normal", "", nil
	}
	if len(args) == 2 && args[0] == "dev" {
		return "dev", args[1], nil
	}
	return "", "", fmt.Errorf("invalid server run mode")
}

func validateDevDirectory(raw string) (string, error) {
	directory, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	for _, required := range []string{"package.json", "src"} {
		info, err := os.Stat(filepath.Join(directory, required))
		if err != nil || (required == "src" && !info.IsDir()) {
			return "", fmt.Errorf("development GUI directory is invalid: missing %s in %s", required, directory)
		}
	}
	return directory, nil
}

func start(mode, devDir string) error {
	if err := loadDotEnv(devDir); err != nil {
		return err
	}
	if state, err := serverruntime.LoadRunningState(context.Background()); err == nil {
		fmt.Printf("server already running at %s\n", state.URL)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	logPath, err := serverruntime.LogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"server", "run"}
	if mode == "dev" {
		args = append(args, "dev", devDir)
	}
	cmd := exec.Command(executable, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach.Configure(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	startedPID := cmd.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state, err := serverruntime.LoadState(); err == nil && state.PID == startedPID && healthy(state) {
			fmt.Printf("server started\nurl: %s\nlocalhost: %s\n", state.URL, state.LocalURL)
			printReachableAddresses(state)
			fmt.Printf("pid: %d\n", state.PID)
			return nil
		}
		select {
		case err := <-exited:
			return fmt.Errorf("daemon exited during startup (%v): %s; inspect %s with solomon server logs", err, startupLogTail(logPath), logPath)
		case <-time.After(50 * time.Millisecond):
		}
	}
	// Only terminate the child created by this invocation, never a PID taken
	// from an unverified stale state file.
	if state, err := serverruntime.LoadState(); err == nil && state.PID == startedPID {
		serverruntime.ForceStop(state)
	} else {
		_ = cmd.Process.Kill()
	}
	return fmt.Errorf("server did not become healthy; inspect with: solomon server logs")
}

func startupLogTail(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "log unavailable"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "log unavailable"
	}
	if info.Size() > 2048 {
		_, _ = file.Seek(-2048, io.SeekEnd)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "log unavailable"
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " | ")
}

func runProcess(mode, devDir string) error {
	if err := loadDotEnv(devDir); err != nil {
		return err
	}
	// The server subcommand returns before main's normal logging setup. The
	// runtime (notably the background MCP connector) logs from goroutines, so
	// initialize logging before starting the HTTP service.
	logging.LogInit(logging.INFO_LOG_LEVEL)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serverruntime.Run(ctx, serverruntime.Options{Mode: mode, DevDir: devDir}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func FormatStatusFields(state serverruntime.State) string {
	body := fmt.Sprintf("pid: %d\nversion: %s\nmode: %s\nvite: %s\n", state.PID, state.Version, state.Mode, state.Vite)
	if state.Mode == "dev" && state.DevDir != "" {
		body += fmt.Sprintf("source: %s\n", state.DevDir)
	}
	body += fmt.Sprintf("started: %s\n", state.StartedAt.Local().Format(time.RFC3339))
	return body
}

func status() {
	state, err := serverruntime.LoadRunningState(context.Background())
	if err != nil || !healthy(state) {
		fmt.Println("server: stopped")
		return
	}
	fmt.Printf("server: running\nurl: %s\nlocalhost: %s\n", state.URL, state.LocalURL)
	printReachableAddresses(state)
	fmt.Print(FormatStatusFields(state))
}

func printReachableAddresses(state serverruntime.State) {
	fmt.Println("addresses:")
	if len(state.Addresses) == 0 {
		fmt.Println("  (none detected)")
		return
	}
	for _, address := range state.Addresses {
		label := address.Kind
		if address.Interface != "" {
			label += " (" + address.Interface + ")"
		}
		fmt.Printf("  %s: %s\n", label, address.URL)
	}
}

func stop() error {
	state, err := serverruntime.LoadRunningState(context.Background())
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, state.URL+"/_solomon/stop", nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("stop verified daemon: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("server refused stop: %s", response.Status)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if _, err := serverruntime.LoadState(); errors.Is(err, os.ErrNotExist) {
			fmt.Println("server stopped")
			return nil
		}
		if !healthy(state) {
			fmt.Println("server stopped")
			return nil
		}
	}
	return fmt.Errorf("daemon %d did not stop; state preserved for recovery", state.PID)
}

func healthy(state serverruntime.State) bool {
	health, err := serverruntime.ReadHealth(context.Background(), state)
	return err == nil && health.OK && health.Server.PID == state.PID && health.Server.StartedAt.Equal(state.StartedAt)
}

func logs(interactive bool) error {
	path, err := serverruntime.LogPath()
	if err != nil {
		return err
	}
	if err := printTail(path); err != nil {
		return err
	}
	if !interactive {
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var offset int64
	if info, err := os.Stat(path); err == nil {
		offset = info.Size()
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			file, err := os.Open(path)
			if err != nil {
				continue
			}
			info, _ := file.Stat()
			if info != nil && info.Size() < offset {
				offset = 0
			}
			_, _ = file.Seek(offset, io.SeekStart)
			n, _ := io.Copy(os.Stdout, file)
			offset += n
			_ = file.Close()
		}
	}
}

func printTail(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no server logs yet")
		}
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	const maxTail = int64(64 * 1024)
	if info.Size() > maxTail {
		_, _ = file.Seek(-maxTail, io.SeekEnd)
	}
	_, err = io.Copy(os.Stdout, file)
	return err
}
