//go:build windows

package agentruntime

import (
	readline "github.com/chzyer/readline"
)

func platformStdin() stdinReadCloser {
	return readline.NewRawReader()
}
