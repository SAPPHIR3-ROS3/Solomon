//go:build !windows

package multiline

import (
	"io"
	"os"
)

type nopCloseStdin struct{ io.Reader }

func (nopCloseStdin) Close() error { return nil }

func platformStdin() stdinReadCloser {
	return nopCloseStdin{Reader: os.Stdin}
}

func PrepareConsoleInput() func() {
	return func() {}
}
