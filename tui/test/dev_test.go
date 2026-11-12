package test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/dev"
)

func init() {
	if os.Getenv("SOLOMON_DEV_TEST_HELPER") != "1" {
		return
	}
	root := os.Getenv("SOLOMON_DEV_TEST_ROOT")
	if len(os.Args) > 1 && os.Args[1] == "build" {
		f, _ := os.OpenFile(filepath.Join(root, "builds"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if f != nil {
			f.WriteString("b")
			f.Close()
		}
		if _, err := os.Stat(filepath.Join(root, "build.fail")); err == nil {
			os.Exit(1)
		}
		executable, executableErr := os.Executable()
		if executableErr != nil {
			os.Exit(2)
		}
		if _, err := os.Stat(filepath.Join(root, "slow")); err == nil {
			time.Sleep(120 * time.Millisecond)
		}
		data, err := os.ReadFile(executable)
		if err != nil {
			os.Exit(2)
		}
		if os.WriteFile(os.Args[3], data, 0700) != nil {
			os.Exit(3)
		}
		if os.Chmod(os.Args[3], 0700) != nil {
			os.Exit(4)
		}
		if _, err := os.Stat(filepath.Join(root, "rollback")); err == nil {
			os.WriteFile(os.Args[3]+".fail", []byte("1"), 0600)
		}
		os.Exit(0)
	}
	executable, _ := os.Executable()
	if _, err := os.Stat(executable + ".fail"); err == nil {
		os.Exit(1)
	}
	state := os.Getenv("SOLOMON_TUI_STATE_PATH")
	request := os.Getenv("SOLOMON_TUI_RELOAD_REQUEST")
	status := os.Getenv("SOLOMON_TUI_DEV_STATUS")
	os.WriteFile(filepath.Join(root, "paths"), []byte(status), 0600)
	f, _ := os.OpenFile(filepath.Join(root, "starts"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if f != nil {
		f.WriteString("s")
		f.Close()
	}
	for {
		os.WriteFile(filepath.Join(root, "heartbeat"), []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0600)
		if _, err := os.Stat(filepath.Join(root, "quit")); err == nil {
			os.Exit(0)
		}
		data, err := os.ReadFile(request)
		_, ignored := os.Stat(filepath.Join(root, "ignore"))
		if err == nil && os.IsNotExist(ignored) {
			var req dev.Request
			if json.Unmarshal(data, &req) == nil && req.Token != "" {
				token := req.Token
				if _, err := os.Stat(filepath.Join(root, "wrong")); err == nil {
					token = "wrong"
				}
				payload, _ := json.Marshal(map[string]any{"version": 1, "reload_token": token, "data": map[string]any{}})
				temp := state + ".tmp"
				os.WriteFile(temp, payload, 0600)
				os.Rename(temp, state)
				os.Exit(0)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func devWait(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("development supervisor condition timed out")
}
func devWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func devRead(path string) string { data, _ := os.ReadFile(path); return string(data) }
func devHarness(t *testing.T) (string, context.CancelFunc, chan error) {
	t.Helper()
	root := t.TempDir()
	bin := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(bin, "go"), data, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GORACE", os.Getenv("GORACE")+" atexit_sleep_ms=0")
	t.Setenv("SOLOMON_DEV_TEST_HELPER", "1")
	t.Setenv("SOLOMON_DEV_TEST_ROOT", root)
	devWrite(t, filepath.Join(root, "main.go"), "one")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- dev.Run(ctx, dev.Options{Directory: root, PollInterval: 15 * time.Millisecond}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("supervisor did not stop")
		}
	})
	devWait(t, func() bool {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("supervisor stopped: %v builds=%q", err, devRead(filepath.Join(root, "builds")))
		default:
		}
		return devRead(filepath.Join(root, "starts")) == "s"
	})
	return root, cancel, done
}
func devStatus(root string) dev.Status {
	var status dev.Status
	json.Unmarshal([]byte(devRead(devRead(filepath.Join(root, "paths")))), &status)
	return status
}
func TestDevBuildFailureKeepsChild(t *testing.T) {
	root, _, _ := devHarness(t)
	before := devRead(filepath.Join(root, "heartbeat"))
	devWrite(t, filepath.Join(root, "build.fail"), "1")
	devWrite(t, filepath.Join(root, "main.go"), "two")
	devWait(t, func() bool { return devStatus(root).Phase == "error" })
	devWait(t, func() bool { return devRead(filepath.Join(root, "heartbeat")) != before })
	if devRead(filepath.Join(root, "starts")) != "s" {
		t.Fatal("build failure replaced active child")
	}
}
func TestDevWatchFilteringAndToken(t *testing.T) {
	root, _, _ := devHarness(t)
	for _, name := range []string{"unused_test.go", "interface.json", "notes.txt"} {
		devWrite(t, filepath.Join(root, name), "ignored")
	}
	for _, name := range []string{".git", "node_modules", ".dev", "test", "tmp"} {
		os.Mkdir(filepath.Join(root, name), 0700)
		devWrite(t, filepath.Join(root, name, "ignored.go"), "ignored")
	}
	time.Sleep(100 * time.Millisecond)
	if devRead(filepath.Join(root, "builds")) != "b" {
		t.Fatal("excluded file caused rebuild")
	}
	os.Mkdir(filepath.Join(root, "fixtures"), 0700)
	devWrite(t, filepath.Join(root, "fixtures", "sample.json"), "{}")
	devWait(t, func() bool { return devRead(filepath.Join(root, "starts")) == "ss" })
	devWait(t, func() bool { return devStatus(root).Generation == 2 })
	statusPath := devRead(filepath.Join(root, "paths"))
	for _, path := range []string{statusPath, filepath.Join(filepath.Dir(statusPath), "state.json")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private file permissions: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(statusPath), "request.json")); !os.IsNotExist(err) {
		t.Fatal("stale request retained")
	}
}
func TestDevWrongTokenRestoresPrevious(t *testing.T) {
	root, _, _ := devHarness(t)
	devWrite(t, filepath.Join(root, "wrong"), "1")
	devWrite(t, filepath.Join(root, "main.go"), "two")
	devWait(t, func() bool { return devRead(filepath.Join(root, "starts")) == "ss" })
	if status := devStatus(root); status.Phase != "error" || status.Generation != 1 {
		t.Fatalf("invalid state accepted: %+v", status)
	}
}
func TestDevInvalidDirectory(t *testing.T) {
	err := dev.Run(context.Background(), dev.Options{Directory: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("missing source directory accepted")
	}
}
func TestDevCancellation(t *testing.T) {
	root, cancel, done := devHarness(t)
	cancel()
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation timed out")
	}
	if devRead(filepath.Join(root, "starts")) != "s" {
		t.Fatal("cancellation started a replacement")
	}
}

func TestDevHandoffTimeoutPreservesChild(t *testing.T) {
	root, _, _ := devHarness(t)
	devWrite(t, filepath.Join(root, "ignore"), "1")
	devWrite(t, filepath.Join(root, "main.go"), "two")
	devWait(t, func() bool { return devStatus(root).Phase == "error" })
	before := devRead(filepath.Join(root, "heartbeat"))
	devWait(t, func() bool { return devRead(filepath.Join(root, "heartbeat")) != before })
	if devRead(filepath.Join(root, "starts")) != "s" {
		t.Fatal("timeout replaced active child")
	}
}
func TestDevCoalescesChangesDuringBuild(t *testing.T) {
	root, _, _ := devHarness(t)
	devWrite(t, filepath.Join(root, "slow"), "1")
	devWrite(t, filepath.Join(root, "main.go"), "two")
	devWait(t, func() bool { return len(devRead(filepath.Join(root, "builds"))) == 2 })
	devWrite(t, filepath.Join(root, "main.go"), "three")
	devWait(t, func() bool { return devRead(filepath.Join(root, "starts")) == "ss" })
	if len(devRead(filepath.Join(root, "builds"))) != 3 {
		t.Fatal("change during build was lost")
	}
}
func TestDevRollsBackEarlyFailure(t *testing.T) {
	root, _, _ := devHarness(t)
	devWrite(t, filepath.Join(root, "rollback"), "1")
	devWrite(t, filepath.Join(root, "main.go"), "two")
	devWait(t, func() bool { return devRead(filepath.Join(root, "starts")) == "ss" })
	if status := devStatus(root); status.Phase != "error" || status.Generation != 1 {
		t.Fatalf("rollback failed: %+v", status)
	}
}
func TestDevUserQuit(t *testing.T) {
	root, _, done := devHarness(t)
	devWrite(t, filepath.Join(root, "quit"), "1")
	select {
	case err := <-done:
		done <- err
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("user quit did not stop supervisor")
	}
}
