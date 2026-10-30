//go:build !windows

package server

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

type unixTerminalProcess struct {
	cmd      *exec.Cmd
	file     *os.File
	waitOnce sync.Once
	waitErr  error
	cleanup  func()
}

func startTerminalProcess(options terminalProcessOptions) (terminalProcess, error) {
	args, env, cleanup, err := terminalIntegration(options.Shell, options.Args, options.Env)
	if err != nil {
		return nil, err
	}
	command := exec.Command(options.Shell, args...)
	command.Dir = options.Cwd
	command.Env = env
	file, err := pty.StartWithSize(command, &pty.Winsize{Cols: options.Cols, Rows: options.Rows})
	if err != nil {
		cleanup()
		return nil, err
	}

	return &unixTerminalProcess{cmd: command, file: file, cleanup: cleanup}, nil
}

func (p *unixTerminalProcess) Read(data []byte) (int, error)  { return p.file.Read(data) }
func (p *unixTerminalProcess) Write(data []byte) (int, error) { return p.file.Write(data) }
func (p *unixTerminalProcess) Close() error                   { return p.file.Close() }

func (p *unixTerminalProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	// PTY children own a session/process group. Stop their workers as well.
	return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
}

func (p *unixTerminalProcess) Resize(cols, rows uint16) error {
	return pty.Setsize(p.file, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixTerminalProcess) Wait() error {
	p.waitOnce.Do(func() { p.waitErr = p.cmd.Wait(); p.cleanup() })
	return p.waitErr
}
