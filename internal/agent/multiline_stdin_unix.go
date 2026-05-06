//go:build !windows

package agent

import (
	"io"
	"os"
)

type nopCloseStdin struct{ io.Reader }

func (nopCloseStdin) Close() error { return nil }

func platformStdin() stdinReadCloser {
	return nopCloseStdin{Reader: os.Stdin}
}
