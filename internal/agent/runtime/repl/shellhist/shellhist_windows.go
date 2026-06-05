//go:build windows

package shellhist

import (
	"os"
	"path/filepath"
)

func psReadLinePath() (string, historyKind) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", historyNone
	}
	if _, err := os.Stat(filepath.Join(appData, "Microsoft", "PowerShell", "PSReadLine")); err == nil {
		return filepath.Join(appData, "Microsoft", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"), historyPlain
	}
	return filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"), historyPlain
}
