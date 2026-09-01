//go:build windows

package server

func startTerminalProcess(_ terminalProcessOptions) (terminalProcess, error) {
	return nil, errTerminalUnsupported
}
