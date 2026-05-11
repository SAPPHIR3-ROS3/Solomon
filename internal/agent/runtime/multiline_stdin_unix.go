//go:build !windows

package agentruntime

import (
	"io"
	"os"
)

type nopCloseStdin struct{ io.Reader }

func (nopCloseStdin) Close() error { return nil }

func platformStdin() stdinReadCloser {
	return nopCloseStdin{Reader: os.Stdin}
}
