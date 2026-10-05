package updater

import (
	"context"
	_ "embed"
	"io"
	"os"
	"os/exec"
)

//go:embed desktop_registration.ps1
var desktopRegistrationScript string

func registerDesktop(ctx context.Context, tag, cli, target string, progress io.Writer) error {
	file, err := os.CreateTemp("", "solomon-register-*.ps1")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(desktopRegistrationScript); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, windowsPowerShellExe(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file.Name(), "-CLIPath", cli, "-DesktopPath", target, "-Version", tag)
	cmd.Stdout, cmd.Stderr = progress, progress
	return cmd.Run()
}
