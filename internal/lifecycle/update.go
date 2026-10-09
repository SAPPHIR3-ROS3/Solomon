package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func RunUpdateCommand(args []string) bool {
	if len(args) < 2 || args[1] != "__upgrade-runtime" {
		return false
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	if len(args) != 4 {
		fmt.Fprintln(os.Stderr, "invalid update coordinator arguments")
		os.Exit(1)
	}
	home, err := paths.SolomonHome()
	if err == nil {
		err = os.MkdirAll(filepath.Join(home, "run"), 0700)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lock := flock.New(filepath.Join(home, "run", "update.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		fmt.Fprintln(os.Stderr, "another Solomon update is already running", err)
		os.Exit(1)
	}
	defer lock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	fmt.Printf("Updating Solomon runtime to %s\n", args[2])
	ops := platformRuntimeUpdateOps(ctx, args[2])
	parentPID, err := strconv.Atoi(args[3])
	if err != nil || parentPID <= 1 {
		fmt.Fprintln(os.Stderr, "invalid update parent")
		os.Exit(1)
	}
	parentIdentity := processIdentity(parentPID)
	stop := ops.stop
	ops.stop = func(target string) error {
		if err := stop(target); err != nil {
			return err
		}
		return waitProcessExit(ctx, parentPID, parentIdentity)
	}
	if err := applyRuntimeUpdate(ctx, args[2], ops); err != nil {
		fmt.Fprintln(os.Stderr, "Solomon update failed:", err)
		os.Exit(1)
	}
	fmt.Println("Solomon daemon and clients restarted.")
	return true
}

type runtimeUpdateOps struct {
	prepare func() (staged, target string, err error)
	stop    func(target string) error
	commit  func(staged, target string) error
	start   func(target string) error
	restore func(target string) error
	quiesce func() error
}

// Keep downloads outside the interruption window and restore the old runtime
// on any failure after stopping it. Tests supply isolated processes and files.
func applyRuntimeUpdate(ctx context.Context, tag string, ops runtimeUpdateOps) (err error) {
	staged, target, err := ops.prepare()
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	if err := ctx.Err(); err != nil {
		return err
	}
	backup, err := updater.BackupInstallation(target)
	if err != nil {
		return err
	}
	rollbackSafe := true
	defer func() {
		if rollbackSafe {
			backup.Close()
		}
	}()
	commitAttempted := false
	defer func() {
		if err != nil {
			if ops.quiesce != nil {
				if stopErr := ops.quiesce(); stopErr != nil {
					rollbackSafe = false
					err = errors.Join(err, stopErr, fmt.Errorf("previous installation backups retained: %v", backup.Locations()))
					return
				}
			}
			if commitAttempted {
				if rollbackErr := backup.Restore(); rollbackErr != nil {
					rollbackSafe = false
					err = errors.Join(err, rollbackErr)
					return
				}
			}
			err = errors.Join(err, ops.restore(target))
		}
	}()
	if err = ops.stop(target); err != nil {
		return err
	}
	commitAttempted = true
	if err = ops.commit(staged, target); err != nil {
		return err
	}
	if err = ops.start(target); err != nil {
		return err
	}
	return nil
}

type desktopUpdateProcess struct {
	pid        int
	identity   string
	executable string
	env        []string
	cwd        string
	args       []string
	stopFile   string
	kind       string
}

func platformRuntimeUpdateOps(ctx context.Context, tag string, defaults ...serverruntime.State) runtimeUpdateOps {
	var desktops []desktopUpdateProcess
	var stopped []desktopUpdateProcess
	var restarted []desktopUpdateProcess
	var oldState serverruntime.State
	if len(defaults) > 0 {
		oldState = defaults[0]
	}
	var daemonStopped bool
	daemonPort := ""
	start := func(target string, requireVersion bool) error {
		startCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		args := []string{"server", "start"}
		if !requireVersion && oldState.Mode == "dev" {
			args = append(args, "dev", oldState.DevDir)
		}
		command := exec.CommandContext(startCtx, target, args...)
		if daemonPort != "" {
			command.Env = append(os.Environ(), "SOLOMON_SERVER_PORT="+daemonPort)
		}
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			state, err := serverruntime.LoadState()
			if err == nil {
				health, err := serverruntime.ReadHealth(startCtx, state)
				if err == nil && health.OK && health.Server.PID == state.PID && (!requireVersion || health.Server.Version == tag) {
					if tag == "" {
						if err := verifyInstalledDaemon(target, health.Server); err != nil {
							return err
						}
					}
					for _, desktop := range stopped {
						executable := desktop.executable
						if tag == "" {
							executable = target
							if desktop.kind != "tui" {
								executable, err = updater.DesktopExecutablePath(target)
								if err != nil {
									return err
								}
							}
						}
						command := exec.Command(executable, desktop.args...)
						command.Env, command.Dir = desktop.env, desktop.cwd
						if daemonPort != "" {
							command.Env = append(command.Env, "SOLOMON_SERVER_PORT="+daemonPort)
						}
						if desktop.kind != "tui" {
							command.Env = append(command.Env, "SOLOMON_BINARY="+target)
						}
						configureClientProcess(command, desktop.kind)
						if err := startClientProcess(command, desktop.kind); err != nil {
							return fmt.Errorf("restart desktop: %w", err)
						}
						client := desktop
						client.executable = executable
						client.pid = command.Process.Pid
						client.identity = processIdentity(client.pid)
						client.stopFile = ""
						restarted = append(restarted, client)
						_ = command.Process.Release()
						if tag == "" {
							if err := waitClientRegistration(startCtx, client); err != nil {
								return err
							}
						}
					}
					stopped = nil
					return nil
				}
			}
			select {
			case <-startCtx.Done():
				return startCtx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		return fmt.Errorf("daemon did not become ready (expected version %q); inspect with solomon server logs", tag)
	}
	return runtimeUpdateOps{
		prepare: func() (string, string, error) { return updater.PrepareInstall(ctx, tag, os.Stdout) },
		stop: func(target string) error {
			var err error
			desktopTarget, err := updater.DesktopExecutablePath(target)
			if err != nil {
				return err
			}
			desktops, err = updateClients(desktopTarget)
			if err != nil {
				return err
			}
			state, err := serverruntime.LoadRunningState(ctx)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil {
				oldState = state
				parsed, err := url.Parse(oldState.URL)
				if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "localhost" && !net.ParseIP(parsed.Hostname()).IsLoopback()) {
					return fmt.Errorf("refusing to stop a non-loopback daemon")
				}
				daemonPort = parsed.Port()
				health, err := serverruntime.ReadHealth(ctx, oldState)
				if err != nil || !health.OK || health.Server.PID != oldState.PID {
					return fmt.Errorf("cannot verify daemon identity (pid %d): %v", oldState.PID, err)
				}
			}
			for _, desktop := range desktops {
				if processIdentity(desktop.pid) != desktop.identity {
					continue
				}
				if err := requestClientStop(desktop); err != nil {
					return err
				}
				// Once requested, shutdown can finish after the update context expires.
				// Recovery must remember this client even if the wait is interrupted.
				stopped = append(stopped, desktop)
				if err := waitProcessExit(ctx, desktop.pid, desktop.identity); err != nil {
					return err
				}
			}
			if oldState.PID > 0 {
				identity := processIdentity(oldState.PID)
				request, _ := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(oldState.URL, "/")+"/_solomon/stop", nil)
				response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
				if err != nil {
					return err
				}
				response.Body.Close()
				if response.StatusCode != http.StatusAccepted {
					return fmt.Errorf("daemon refused to stop: %s", response.Status)
				}
				daemonStopped = true
				if err := waitProcessExit(ctx, oldState.PID, identity); err != nil {
					return err
				}
			}
			return nil
		},
		commit: updater.CommitInstall,
		start:  func(target string) error { return start(target, tag != "") },
		quiesce: func() error {
			// Finish pending shutdowns with an independent recovery deadline before
			// restoring files or starting replacements for those same clients.
			for _, client := range stopped {
				if err := waitProcessExit(context.Background(), client.pid, client.identity); err != nil {
					return err
				}
			}
			if daemonStopped {
				if err := waitProcessExit(context.Background(), oldState.PID, processIdentity(oldState.PID)); err != nil {
					return err
				}
			}
			// Close any partially restarted clients before restoring mapped binaries.
			for _, client := range restarted {
				clients, lookupErr := updateClients(filepath.Join(filepath.Dir(client.executable), desktopName()))
				if lookupErr != nil {
					return lookupErr
				}
				for _, registered := range clients {
					if registered.pid == client.pid && registered.identity == client.identity {
						client = registered
						break
					}
				}
				if err := requestClientStop(client); err != nil {
					return err
				}
				if err := waitProcessExit(context.Background(), client.pid, client.identity); err != nil {
					return err
				}
			}
			restarted = nil
			// Windows cannot replace an executable while its process is running.
			if state, err := serverruntime.LoadState(); err == nil && state.PID != oldState.PID {
				request, _ := http.NewRequest(http.MethodPost, strings.TrimRight(state.URL, "/")+"/_solomon/stop", nil)
				if response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request); err == nil {
					response.Body.Close()
				}
				if err := waitProcessExit(context.Background(), state.PID, processIdentity(state.PID)); err != nil {
					return err
				}
			}
			return nil
		},
		restore: func(target string) error {
			if daemonStopped || len(stopped) > 0 {
				return start(target, false)
			}
			return nil
		},
	}
}

func waitProcessExit(ctx context.Context, pid int, identity string) error {
	deadline := time.Now().Add(20 * time.Second)
	for identity != "" && processIdentity(pid) == identity {
		if time.Now().After(deadline) {
			return fmt.Errorf("Solomon process %d did not stop", pid)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}

// ApplyLocalUpdate applies a verified local build using the same runtime handoff.
func ApplyLocalUpdate(ctx context.Context, tag, staged, target string, desktopStaged ...string) error {
	ops := platformRuntimeUpdateOps(ctx, tag)
	ops.prepare = func() (string, string, error) { return staged, target, nil }
	if len(desktopStaged) > 0 {
		desktopTarget, err := updater.DesktopExecutablePath(target)
		if err != nil {
			return err
		}
		backup, err := updater.BackupInstallation(desktopTarget)
		if err != nil {
			return err
		}
		safeToClean := false
		defer func() {
			if safeToClean {
				backup.Close()
			}
		}()
		commit, restore := ops.commit, ops.restore
		desktopCommitted := false
		ops.commit = func(staged, target string) error {
			if err := commit(staged, target); err != nil {
				return err
			}
			err := updater.CommitInstall(desktopStaged[0], desktopTarget)
			desktopCommitted = err == nil
			return err
		}
		ops.restore = func(target string) error {
			if desktopCommitted {
				if err := backup.Restore(); err != nil {
					return err
				}
			}
			safeToClean = true
			return restore(target)
		}
		err = applyRuntimeUpdate(ctx, tag, ops)
		if err == nil {
			safeToClean = true
		}
		return err
	}
	return applyRuntimeUpdate(ctx, tag, ops)
}

func desktopName() string {
	if runtime.GOOS == "windows" {
		return "solomon-desktop.exe"
	}
	return "solomon-desktop"
}
