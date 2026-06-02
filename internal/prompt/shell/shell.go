package shell

import (
	"os"
	"runtime"
	"strings"
)

func Effective() string {
	if v := strings.TrimSpace(os.Getenv("SHELL")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return windowsEffective()
	}
	if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
		return v
	}
	return "unknown"
}
