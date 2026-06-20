//go:build !windows

package input

import (
	"os"

	"golang.org/x/term"
)

func OpenTerminal() (*os.File, error) {
	if f, err := os.Open("/dev/tty"); err == nil {
		if term.IsTerminal(int(f.Fd())) {
			return f, nil
		}
		_ = f.Close()
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, nil
	}
	return nil, os.ErrInvalid
}
