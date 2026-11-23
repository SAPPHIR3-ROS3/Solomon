package desktopgit

import (
	"fmt"
	"os/exec"
)

func RequireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("Git executable not found in PATH")
	}
	return nil
}
