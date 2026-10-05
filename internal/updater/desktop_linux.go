package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
)

func registerDesktop(ctx context.Context, tag, cli, target string, progress io.Writer) error {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	iconPath := filepath.Join(dataHome, "icons", "hicolor", "512x512", "apps", "solomon.png")
	if err := os.MkdirAll(filepath.Dir(iconPath), 0o755); err != nil {
		return err
	}
	icon, err := logo.IconPNG(512)
	if err != nil {
		return err
	}
	if err := os.WriteFile(iconPath, icon, 0o644); err != nil {
		return err
	}
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(target)
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Solomon\nComment=AI coding assistant\nExec=\"%s\"\nIcon=solomon\nTerminal=false\nCategories=Development;\nStartupWMClass=Solomon\n", escaped)
	if err := os.WriteFile(filepath.Join(dir, "solomon.desktop"), []byte(entry), 0o644); err != nil {
		return err
	}
	if command, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.CommandContext(ctx, command, dir).Run()
	}
	fmt.Fprintln(progress, "Solomon registered in the application menu.")
	return nil
}
