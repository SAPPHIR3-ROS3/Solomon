//go:build windows

package agent

import (
	readline "github.com/chzyer/readline"
)

func platformStdin() stdinReadCloser {
	return readline.NewRawReader()
}
