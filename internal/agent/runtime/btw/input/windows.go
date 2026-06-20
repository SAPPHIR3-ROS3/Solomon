//go:build windows

package input

import (
	"os"

	"golang.org/x/term"
)

func OpenTerminal() (*os.File, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, nil
	}
	return nil, os.ErrInvalid
}
