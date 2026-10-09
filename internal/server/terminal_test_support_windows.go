//go:build windows

package server

// StartTerminalProcessForTest exercises ConPTY with redirected host handles.
func StartTerminalProcessForTest(shell string, args []string, cwd string) (terminalProcess, error) {
	return startTerminalProcess(terminalProcessOptions{
		Shell: shell, Args: args, Cwd: cwd, Cols: 80, Rows: 24,
	})
}
