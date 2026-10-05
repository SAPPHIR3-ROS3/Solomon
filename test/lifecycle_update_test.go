package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
	"github.com/gorilla/websocket"
)

func TestCoordinatedUpdateInstallsBeforeRestartAndRollsBackFailures(t *testing.T) {
	for _, failure := range []string{"", "prepare", "stop", "commit", "start"} {
		t.Run("failure="+failure, func(t *testing.T) {
			dir := t.TempDir()
			name := "solomon"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			target, staged := filepath.Join(dir, name), filepath.Join(dir, "staged")
			if err := os.WriteFile(target, []byte("old runtime"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(staged, []byte("new runtime"), 0700); err != nil {
				t.Fatal(err)
			}
			var events []string
			step := func(name string) error {
				events = append(events, name)
				if failure == name {
					return errors.New(name + " failed")
				}
				return nil
			}
			ops := lifecycle.UpdateOpsForTest{
				Prepare: func() (string, string, error) { return staged, target, step("prepare") },
				Stop:    func(string) error { return step("stop") },
				Commit: func(staged, target string) error {
					if err := step("commit"); err != nil {
						return err
					}
					return updater.CommitInstall(staged, target)
				},
				Start: func(target string) error {
					data, err := os.ReadFile(target)
					if err != nil || string(data) != "new runtime" {
						t.Fatalf("restart before install: %s %v", data, err)
					}
					return step("start")
				},
				Quiesce: func() error { return step("quiesce") },
				Restore: func(target string) error {
					data, err := os.ReadFile(target)
					if err != nil || string(data) != "old runtime" {
						t.Fatalf("restore without old binary: %s %v", data, err)
					}
					return step("restore")
				},
			}
			err := lifecycle.ApplyUpdateForTest(context.Background(), "release", ops)
			if (err != nil) != (failure != "") {
				t.Fatalf("update result: %v", err)
			}
			want := map[string][]string{
				"":        {"prepare", "stop", "commit", "start"},
				"prepare": {"prepare"},
				"stop":    {"prepare", "stop", "quiesce", "restore"},
				"commit":  {"prepare", "stop", "commit", "quiesce", "restore"},
				"start":   {"prepare", "stop", "commit", "start", "quiesce", "restore"},
			}[failure]
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("events=%v want=%v", events, want)
			}
			data, _ := os.ReadFile(target)
			if failure == "" && string(data) != "new runtime" {
				t.Fatalf("installed=%s", data)
			}
			if failure != "" && string(data) != "old runtime" {
				t.Fatalf("failed update replaced runtime: %s", data)
			}
		})
	}
}

func TestCoordinatedUpdateRestartsAnIsolatedDaemonAndNotifiesItsTerminal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("SOLOMON_HOME", filepath.Join(dir, "home"))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	t.Setenv("SOLOMON_SERVER_PORT", fmt.Sprint(port))
	const tag = "v2099.1.0"
	name := "solomon"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target, staged := filepath.Join(dir, name), filepath.Join(dir, "staged")
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags=-X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version="+tag, "-o", target, "./cmd/solomon")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v %s", err, output)
	}
	// This test exercises daemon updates with an already provisioned desktop.
	// The fictional release must never trigger a public release download.
	desktop, err := updater.DesktopExecutablePath(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(desktop), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop, []byte("desktop fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	marker, err := json.Marshal(map[string]string{"tag": tag, "cli": target})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop+".install.json", marker, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, data, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(target, "server", "start")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start fixture: %v %s", err, output)
	}
	state, err := serverruntime.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if state, err := serverruntime.LoadState(); err == nil {
			identity := lifecycle.ProcessIdentityForTest(state.PID)
			request, _ := http.NewRequest(http.MethodPost, state.URL+"/_solomon/stop", nil)
			if response, err := http.DefaultClient.Do(request); err == nil {
				response.Body.Close()
			}
			_ = lifecycle.WaitProcessExitForTest(context.Background(), state.PID, identity)
		}
	}()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(state.URL, "http")+"/__solomon/terminal?path="+dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, _, err := connection.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := connection.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	restartNotice := make(chan bool, 1)
	go func() {
		for {
			_, data, err := connection.ReadMessage()
			if err != nil {
				restartNotice <- false
				return
			}
			if strings.Contains(string(data), "solomon-restarting") {
				restartNotice <- true
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := lifecycle.ApplyLocalUpdate(ctx, tag, staged, target); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-restartNotice:
		if !got {
			t.Fatal("terminal closed without a restart notification")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	updated, err := serverruntime.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if updated.PID == state.PID || updated.Version != tag || updated.Mode != "normal" {
		t.Fatalf("runtime not replaced: %+v", updated)
	}
	if got := serverruntime.RunningVersion(); got != tag {
		t.Fatalf("daemon version=%q", got)
	}
	version, err := exec.Command(target, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), tag) {
		t.Fatalf("CLI not aligned: %s %v", version, err)
	}
}

func TestCoordinatedUpdatePreservesBackupWhenNewRuntimeCannotStop(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "solomon")
	staged := filepath.Join(directory, "staged")
	for path, content := range map[string]string{target: "old", staged: "new"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	restored := false
	err := lifecycle.ApplyUpdateForTest(context.Background(), "release", lifecycle.UpdateOpsForTest{
		Prepare: func() (string, string, error) { return staged, target, nil }, Stop: func(string) error { return nil }, Commit: updater.CommitInstall,
		Start: func(string) error { return errors.New("new daemon failed") }, Quiesce: func() error { return errors.New("new process still holds executable") }, Restore: func(string) error { restored = true; return nil },
	})
	if err == nil || restored {
		t.Fatalf("unsafe restore: err=%v restored=%v", err, restored)
	}
	backup, err := os.ReadFile(target + ".runtime-backup")
	if err != nil || string(backup) != "old" {
		t.Fatalf("lost recoverable binary: %s %v", backup, err)
	}
}
