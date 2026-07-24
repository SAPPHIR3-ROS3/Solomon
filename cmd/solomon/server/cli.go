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
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server/detach"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func Run(args []string) {
	if len(args) == 0 {
		usage()
		return
	}
	switch args[0] {
	case "start":
		mode, devDir, err := parseStart(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if err := start(mode, devDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	case "run":
		mode, devDir, err := parseRun(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		runProcess(mode, devDir)
	case "status":
		status()
	case "stop":
		if err := stop(); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("server: stopped")
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	case "restart":
		mode, devDir := "normal", ""
		if state, err := serverruntime.LoadState(); err == nil {
			mode, devDir = state.Mode, state.DevDir
		}
		if err := stop(); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if err := start(mode, devDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	case "logs":
		interactive := len(args) == 2 && args[1] == "interactive"
		if len(args) > 2 || (len(args) == 2 && !interactive) {
			fmt.Fprintln(os.Stderr, "usage: solomon server logs [interactive]")
			return
		}
		if err := logs(interactive); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	default:
		usage()
	}
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
	if state, err := serverruntime.LoadState(); err == nil {
		if healthy(state) {
			return fmt.Errorf("server already running at %s", state.URL)
		}
		_ = serverruntime.ClearState()
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
	_ = cmd.Process.Release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state, err := serverruntime.LoadState(); err == nil && healthy(state) {
			fmt.Printf("server started\nurl: %s\nlocalhost: %s\npid: %d\n", state.URL, state.LocalURL, state.PID)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server did not become healthy; inspect with: solomon server logs")
}

func runProcess(mode, devDir string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serverruntime.Run(ctx, serverruntime.Options{Mode: mode, DevDir: devDir}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
	}
}

func status() {
	state, err := serverruntime.LoadState()
	if err != nil || !healthy(state) {
		fmt.Println("server: stopped")
		return
	}
	fmt.Printf("server: running\nurl: %s\nlocalhost: %s\npid: %d\nversion: %s\nmode: %s\nvite: %s\nstarted: %s\n", state.URL, state.LocalURL, state.PID, state.Version, state.Mode, state.Vite, state.StartedAt.Local().Format(time.RFC3339))
}

func stop() error {
	state, err := serverruntime.LoadState()
	if err != nil {
		return os.ErrNotExist
	}
	request, err := http.NewRequest(http.MethodPost, state.URL+"/_solomon/stop", nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		_ = serverruntime.ClearState()
		return fmt.Errorf("server was not reachable; cleared stale state")
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
	}
	return fmt.Errorf("server is still stopping")
}

func healthy(state serverruntime.State) bool {
	response, err := (&http.Client{Timeout: 300 * time.Millisecond}).Get(state.URL + "/health")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
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
