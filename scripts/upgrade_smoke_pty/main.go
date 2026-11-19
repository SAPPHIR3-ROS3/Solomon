//go:build unix

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func run(args []string) (int, error) {
	if len(args) == 0 {
		return 1, errors.New("usage: upgrade_smoke_pty command [args...]")
	}
	cmd := exec.Command(args[0], args[1:]...)
	terminal, err := pty.Start(cmd)
	if err != nil {
		return 1, err
	}
	defer terminal.Close()
	if state, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
		defer term.Restore(int(os.Stdin.Fd()), state)
	}
	go io.Copy(terminal, os.Stdin)
	_, copyErr := io.Copy(os.Stdout, terminal)
	if copyErr != nil && !errors.Is(copyErr, syscall.EIO) {
		terminal.Close()
	}
	err = cmd.Wait()
	if err == nil {
		if copyErr != nil && !errors.Is(copyErr, syscall.EIO) {
			return 1, copyErr
		}
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return (-int(status.Signal())) & 255, nil
		}
		return exit.ExitCode(), nil
	}
	return 1, err
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
