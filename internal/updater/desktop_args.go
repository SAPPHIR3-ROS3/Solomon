package updater

import (
	"path/filepath"
	"strings"
)

// DesktopRestartArgs keeps an embedded desktop from reopening as a terminal.
func DesktopRestartArgs(executable string) []string {
	switch strings.ToLower(filepath.Base(executable)) {
	case "solomon", "solomon.exe", "solomon-standalone", "solomon-standalone.exe":
		return []string{"desktop"}
	default:
		return nil
	}
}
