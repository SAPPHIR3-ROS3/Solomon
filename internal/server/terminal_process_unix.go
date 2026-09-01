//go:build !windows

package server

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type unixTerminalProcess struct {
	cmd      *exec.Cmd
	file     *os.File
	waitOnce sync.Once
	waitErr  error
}

func startTerminalProcess(options terminalProcessOptions) (terminalProcess, error) {
	command := exec.Command(options.Shell, options.Args...)
	command.Dir = options.Cwd
	command.Env = options.Env
	file, err := pty.StartWithSize(command, &pty.Winsize{Cols: options.Cols, Rows: options.Rows})
	if err != nil {
		return nil, err
	}
	return &unixTerminalProcess{cmd: command, file: file}, nil
}

func (p *unixTerminalProcess) Read(data []byte) (int, error)  { return p.file.Read(data) }
func (p *unixTerminalProcess) Write(data []byte) (int, error) { return p.file.Write(data) }
func (p *unixTerminalProcess) Close() error                   { return p.file.Close() }

func (p *unixTerminalProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *unixTerminalProcess) Resize(cols, rows uint16) error {
	return pty.Setsize(p.file, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixTerminalProcess) Wait() error {
	p.waitOnce.Do(func() { p.waitErr = p.cmd.Wait() })
	return p.waitErr
}
