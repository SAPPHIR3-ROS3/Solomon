package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func clearTerminal(d Deps) error {
	if runtime.GOOS == "windows" && !ansiClearSupportedOnWindows() {
		PrintSystem(d.Out, "Pulizia schermo non disponibile in cmd.exe; usa Windows Terminal o PowerShell.")
		return nil
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	return nil
}

func ansiClearSupportedOnWindows() bool {
	if strings.TrimSpace(os.Getenv("WT_SESSION")) != "" {
		return true
	}
	sh := prompt.EffectiveShell()
	if sh == "unknown" {
		return false
	}
	return !strings.EqualFold(filepath.Base(sh), "cmd.exe")
}

func Clear(d Deps) error {
	if err := clearTerminal(d); err != nil {
		return err
	}
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	return nil
}
