package commands

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func Clear(d Deps) error {
	if runtime.GOOS == "windows" {
		sh := prompt.EffectiveShell()
		if sh != "unknown" && strings.EqualFold(filepath.Base(sh), "cmd.exe") {
			PrintSystem(d.Out, "Pulizia schermo non disponibile in cmd.exe; usa Windows Terminal o PowerShell.")
			return nil
		}
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	return nil
}
