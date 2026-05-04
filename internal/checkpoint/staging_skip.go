package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
)

func SkipStagingIfRunningExecutable(abs string) bool {
	if abs == "" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exeAbs, err := filepath.Abs(exe)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(abs)
	if err != nil {
		return false
	}
	exeAbs = filepath.Clean(exeAbs)
	target = filepath.Clean(target)
	return strings.EqualFold(exeAbs, target)
}
