package test

import (
	"context"
	"errors"
	"fmt"
	"net"
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
)

func TestHotInstallClientHelper(t *testing.T) {
	if os.Getenv("SOLOMON_HOT_INSTALL_HELPER") != "1" {
		return
	}
	stop, cleanup, err := lifecycle.RegisterClient(context.Background(), "desktop", []string{"-test.run=^TestHotInstallClientHelper$"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	select {
	case <-stop:
		if os.Getenv("SOLOMON_HOT_INSTALL_DELAY_EXIT") == "1" {
			time.Sleep(250 * time.Millisecond)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("no stop request")
	}
}

func TestHotInstallReplacesIsolatedDaemonAndReopensRegisteredDesktop(t *testing.T) {
	runHotInstallWithRegisteredDesktop(t, false)
}

func TestHotInstallCancellationDuringClientShutdownRestoresClient(t *testing.T) {
	runHotInstallWithRegisteredDesktop(t, true)
}

func runHotInstallWithRegisteredDesktop(t *testing.T, cancelDuringStop bool) {
	t.Helper()
	buildEnv := os.Environ()
	dir := t.TempDir()
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", dir)
	}
	t.Setenv("SOLOMON_HOME", filepath.Join(dir, "home"))
	t.Setenv("SOLOMON_HOT_INSTALL_HELPER", "1")
	if cancelDuringStop {
		t.Setenv("SOLOMON_HOT_INSTALL_DELAY_EXIT", "1")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOLOMON_SERVER_PORT", fmt.Sprint(listener.Addr().(*net.TCPAddr).Port))
	listener.Close()
	cliName := "solomon"
	if runtime.GOOS == "windows" {
		cliName += ".exe"
	}
	target, staged := filepath.Join(dir, cliName), filepath.Join(dir, "new-"+cliName)
	for _, build := range []struct{ file, tree string }{{target, "old-fixture"}, {staged, "new-fixture"}} {
		command := exec.Command("go", "build", "-buildvcs=false", "-ldflags=-X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.sourceTree="+build.tree, "-o", build.file, "./cmd/solomon")
		command.Dir = ".."
		command.Env = buildEnv
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, output)
		}
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := updater.DesktopExecutablePath(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(desktop), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop, data, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(target, "server", "start")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start: %v %s", err, output)
	}
	old, err := serverruntime.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if output, err := exec.Command(target, "server", "stop").CombinedOutput(); err != nil {
			t.Logf("cleanup: %v %s", err, output)
		}
	})
	client := exec.Command(desktop, "-test.run=^TestHotInstallClientHelper$")
	client.Env, client.Dir = os.Environ(), dir
	if err := client.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Process.Kill(); _ = client.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, _ := os.ReadDir(filepath.Join(dir, "home", "run", "clients"))
		ready := false
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				ready = true
			}
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client did not register")
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if cancelDuringStop {
		go func() {
			for ctx.Err() == nil {
				entries, _ := filepath.Glob(filepath.Join(dir, "home", "run", "clients", "*.stop"))
				if len(entries) > 0 {
					cancel()
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	err = lifecycle.HotInstall(ctx, target, "unused-default-gui", func() error {
		if cancelDuringStop {
			t.Error("installation ran after cancellation during shutdown")
		}
		if lifecycle.ProcessIdentityForTest(old.PID) != "" || lifecycle.ProcessIdentityForTest(client.Process.Pid) != "" {
			t.Error("install ran while old processes were still alive")
		}
		return updater.CommitInstall(staged, target)
	})
	// Clean up only fixture client processes, including the reopened client.
	t.Cleanup(func() {
		clients, _ := lifecycle.UpdateClientsForTest(desktop)
		for _, p := range clients {
			_ = lifecycle.StopClientForTest(desktop, p.PID)
			_ = lifecycle.WaitProcessExitForTest(context.Background(), p.PID, p.Identity)
		}
	})
	if cancelDuringStop && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !cancelDuringStop && err != nil {
		t.Fatal(err)
	}
	updated, err := serverruntime.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if cancelDuringStop {
		if updated.PID != old.PID || updated.SourceTree != "old-fixture" {
			t.Fatalf("cancelled shutdown replaced the original daemon: %+v", updated)
		}
	} else if updated.PID == old.PID || updated.SourceTree != "new-fixture" || updated.Mode != "normal" {
		t.Fatalf("wrong daemon: %+v", updated)
	}
	clients, err := lifecycle.UpdateClientsForTest(desktop)
	if err != nil || len(clients) != 1 || clients[0].PID == client.Process.Pid {
		t.Fatalf("wrong desktop clients: %+v %v", clients, err)
	}
}

func TestHotInstallOrdersShutdownInstallAndRestartAndRecoversFailures(t *testing.T) {
	for _, failure := range []string{"", "stop", "install", "start", "quiesce"} {
		t.Run(failure, func(t *testing.T) {
			var steps []string
			step := func(name string) error {
				steps = append(steps, name)
				if name == failure || (failure == "quiesce" && name == "start") {
					return errors.New(name + " failed")
				}
				return nil
			}
			err := lifecycle.HotInstallForTest("solomon", func() error { return step("install") }, lifecycle.UpdateOpsForTest{
				Stop:    func(string) error { return step("stop") },
				Start:   func(string) error { return step("start") },
				Quiesce: func() error { return step("quiesce") },
				Restore: func(string) error { return step("restore") },
			})
			want := map[string][]string{
				"":        {"stop", "install", "start"},
				"stop":    {"stop", "quiesce", "restore"},
				"install": {"stop", "install", "quiesce", "restore"},
				"start":   {"stop", "install", "start", "quiesce", "restore"},
				"quiesce": {"stop", "install", "start", "quiesce"},
			}[failure]
			if !reflect.DeepEqual(steps, want) || (err != nil) != (failure != "") {
				t.Fatalf("steps=%v want=%v err=%v", steps, want, err)
			}
		})
	}
}

func TestHotInstallRejectsConcurrentUpdater(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOLOMON_HOME", dir)
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", dir)
	}
	t.Setenv("SOLOMON_SERVER_PORT", fmt.Sprint(freeServerPortForTest(t)))
	called := false
	err := lifecycle.HotInstall(context.Background(), "missing-binary", "missing-gui", func() error {
		called = true
		if err := lifecycle.HotInstall(context.Background(), "missing-binary", "missing-gui", func() error { t.Fatal("nested install ran"); return nil }); err == nil {
			t.Fatal("nested update acquired the same lock")
		}
		return errors.New("fixture install failed")
	})
	if !called || err == nil {
		t.Fatalf("called=%v err=%v", called, err)
	}
}
