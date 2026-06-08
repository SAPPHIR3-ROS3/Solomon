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

func PrepareConsoleOutput() func() {
	return func() {}
}

func EditorUsesRawStdout() bool {
	return false
}

func EditorStdout(fallback io.Writer) io.Writer {
	if fallback == nil {
		return os.Stdout
	}
	return fallback
}
