package dev

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Options struct {
	Directory    string
	Arguments    []string
	WatchRoots   []string
	PollInterval time.Duration
}
type child struct {
	command *exec.Cmd
	done    chan error
}
type buildResult struct {
	path string
	err  error
}

func build(ctx context.Context, directory, work string) buildResult {
	f, err := os.CreateTemp(work, "candidate-*")
	if err != nil {
		return buildResult{err: err}
	}
	path := f.Name()
	if err = f.Close(); err != nil {
		return buildResult{err: err}
	}
	command := exec.CommandContext(ctx, "go", "build", "-o", path, ".")
	command.Dir = directory
	diagnostics := &buildLog{}
	command.Stdout = diagnostics
	command.Stderr = diagnostics
	if err = command.Run(); err != nil {
		os.Remove(path)
		message := strings.TrimSpace(string(diagnostics.data))
		if message == "" {
			message = err.Error()
		}
		return buildResult{err: fmt.Errorf("frontend build failed: %s", message)}
	}
	return buildResult{path: path}
}
func start(ctx context.Context, options Options, binary, work string) (*child, error) {
	if err := os.Remove(filepath.Join(work, "request.json")); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command := exec.Command(binary, options.Arguments...)
	command.Dir = options.Directory
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "SOLOMON_TUI_STATE_PATH=") && !strings.HasPrefix(env, "SOLOMON_TUI_RELOAD_REQUEST=") && !strings.HasPrefix(env, "SOLOMON_TUI_DEV_STATUS=") {
			command.Env = append(command.Env, env)
		}
	}
	command.Env = append(command.Env, "SOLOMON_TUI_STATE_PATH="+filepath.Join(work, "state.json"), "SOLOMON_TUI_RELOAD_REQUEST="+filepath.Join(work, "request.json"), "SOLOMON_TUI_DEV_STATUS="+filepath.Join(work, "status.json"))
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &child{command: command, done: make(chan error, 1)}
	go func() { process.done <- command.Wait() }()
	return process, nil
}
func stop(process *child) {
	if process == nil {
		return
	}
	select {
	case <-process.done:
		return
	default:
	}
	_ = process.command.Process.Signal(os.Interrupt)
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-process.done:
	case <-timer.C:
		_ = process.command.Process.Kill()
		<-process.done
	}
}

func Run(parent context.Context, options Options) error {
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if options.Directory == "" {
		options.Directory = "."
	}
	directory, err := filepath.Abs(options.Directory)
	if err != nil {
		return err
	}
	options.Directory = directory
	if len(options.WatchRoots) == 0 {
		options.WatchRoots = []string{directory}
	} else {
		options.WatchRoots = append([]string(nil), options.WatchRoots...)
		for i, root := range options.WatchRoots {
			if !filepath.IsAbs(root) {
				options.WatchRoots[i] = filepath.Join(directory, root)
			}
		}
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 150 * time.Millisecond
	}
	baseline, err := snapshot(options.WatchRoots)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "solomon-tui-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err = os.Chmod(work, 0700); err != nil {
		return err
	}
	generation := 1
	status := func(phase, message string) error {
		return atomicJSON(filepath.Join(work, "status.json"), Status{Phase: phase, Message: message, Generation: generation})
	}
	if err = status("building", ""); err != nil {
		return err
	}
	first := build(ctx, directory, work)
	if first.err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return first.err
	}
	current := first.path
	active, err := start(ctx, options, current, work)
	if err != nil {
		return err
	}
	defer func() { stop(active) }()
	if err = status("ready", ""); err != nil {
		return err
	}
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	results := make(chan buildResult, 1)
	building := false
	dirty := false
	candidate := ""
	token := ""
	previous := ""
	var deadline <-chan time.Time
	var handoff *time.Timer
	defer func() {
		if handoff != nil {
			handoff.Stop()
		}
		cancel()
		if building {
			<-results
		}
	}()
	var probation <-chan time.Time
	var guard *time.Timer
	defer func() {
		if guard != nil {
			guard.Stop()
		}
	}()
	launchBuild := func() error {
		if err := status("building", ""); err != nil {
			return err
		}
		building = true
		dirty = false
		go func() { results <- build(ctx, directory, work) }()
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			next, scanErr := snapshot(options.WatchRoots)
			if scanErr != nil {
				if err = status("error", "source scan failed"); err != nil {
					return err
				}
				continue
			}
			if !equal(baseline, next) {
				baseline = next
				dirty = true
			}
			if dirty && !building && token == "" && probation == nil {
				if candidate != "" {
					os.Remove(candidate)
					candidate = ""
				}
				if err = launchBuild(); err != nil {
					return err
				}
			}
		case result := <-results:
			building = false
			if result.err != nil {
				if err = status("error", result.err.Error()); err != nil {
					return err
				}
				continue
			}
			if dirty {
				os.Remove(result.path)
				if err = launchBuild(); err != nil {
					return err
				}
				continue
			}
			candidate = result.path
			token, err = nonce()
			if err != nil {
				return err
			}
			if err = atomicJSON(filepath.Join(work, "request.json"), Request{Token: token}); err != nil {
				return err
			}
			if err = status("handoff", ""); err != nil {
				return err
			}
			handoff = time.NewTimer(3 * time.Second)
			deadline = handoff.C
		case <-deadline:
			deadline = nil
			token = ""
			if err = os.Remove(filepath.Join(work, "request.json")); err != nil && !os.IsNotExist(err) {
				return err
			}
			os.Remove(candidate)
			candidate = ""
			if err = status("error", "reload handoff timed out; active frontend preserved"); err != nil {
				return err
			}
		case exitErr := <-active.done:
			active = nil
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if token != "" {
				handoff.Stop()
				deadline = nil
				if err = verifyState(filepath.Join(work, "state.json"), token); err != nil {
					token = ""
					os.Remove(candidate)
					candidate = ""
					if err = status("error", "reload state rejected; restoring previous frontend"); err != nil {
						return err
					}
					active, err = start(ctx, options, current, work)
					if err != nil {
						return err
					}
					continue
				}
				token = ""
				previous = current
				current = candidate
				candidate = ""
				generation++
				active, err = start(ctx, options, current, work)
				if err != nil {
					current = previous
					previous = ""
					generation--
					if err = status("error", "candidate could not start; restoring previous frontend"); err != nil {
						return err
					}
					active, err = start(ctx, options, current, work)
					if err != nil {
						return err
					}
					continue
				}
				guard = time.NewTimer(750 * time.Millisecond)
				probation = guard.C
				if err = status("ready", ""); err != nil {
					return err
				}
				continue
			}
			if probation != nil && exitErr != nil && previous != "" {
				guard.Stop()
				probation = nil
				os.Remove(current)
				current = previous
				previous = ""
				generation--
				if err = status("error", "candidate exited early; restoring previous frontend"); err != nil {
					return err
				}
				active, err = start(ctx, options, current, work)
				if err != nil {
					return err
				}
				continue
			}
			if exitErr != nil {
				return fmt.Errorf("frontend exited: %w", exitErr)
			}
			return nil
		case <-probation:
			probation = nil
			if previous != "" {
				os.Remove(previous)
				previous = ""
			}
		}
	}
}

type buildLog struct{ data []byte }

func (b *buildLog) Write(data []byte) (int, error) {
	count := len(data)
	b.data = append(b.data, data...)
	if len(b.data) > 16<<10 {
		b.data = append([]byte(nil), b.data[len(b.data)-(16<<10):]...)
	}
	return count, nil
}
