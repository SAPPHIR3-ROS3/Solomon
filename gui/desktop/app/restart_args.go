package app

import (
	"os"
	"path/filepath"
	"strings"
)

func desktopRestartArgs() []string {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	if base == "solomon" || base == "solomon.exe" {
		return []string{"desktop"}
	}
	return nil
}
