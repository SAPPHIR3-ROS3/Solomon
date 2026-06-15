package clipboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrNoImage = errors.New("no image in clipboard")

const (
	psLoadWinForms = `Add-Type -AssemblyName System.Windows.Forms; `
	psLoadDrawing  = `Add-Type -AssemblyName System.Drawing; `
)

func HasImage() bool {
	switch runtime.GOOS {
	case "darwin":
		return hasImageDarwin()
	case "linux":
		return hasImageLinux()
	case "windows":
		return hasImageWindows()
	}
	return false
}

func hasImageDarwin() bool {
	info, err := exec.Command("osascript", "-e", "clipboard info").Output()
	if err != nil {
		return false
	}
	s := string(info)
	return strings.Contains(s, "PNGf") || strings.Contains(s, "JPEG") || strings.Contains(s, "TIFF")
}

func hasImageLinux() bool {
	if _, err := exec.LookPath("wl-paste"); err == nil {
		out, err := exec.Command("wl-paste", "--list-types").Output()
		if err == nil && strings.Contains(string(out), "image") {
			return true
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
		if err == nil && (strings.Contains(string(out), "image/png") || strings.Contains(string(out), "image/jpeg")) {
			return true
		}
	}
	return false
}

func hasImageWindows() bool {
	ps := psLoadWinForms +
		`if ([System.Windows.Forms.Clipboard]::ContainsImage()) { exit 0 } else { exit 1 }`
	err := exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
	return err == nil
}

func PasteImage(dir, chatID string, seq int) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create images dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.%d.png", chatID, seq))
	switch runtime.GOOS {
	case "darwin":
		return path, pasteImageDarwin(path)
	case "linux":
		return path, pasteImageLinux(path)
	case "windows":
		return path, pasteImageWindows(path)
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func pasteImageDarwin(path string) error {
	script := fmt.Sprintf(`
set imgData to the clipboard as «class PNGf»
set outFile to open for access POSIX file %q with write permission
write imgData to outFile
close access outFile
`, path)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("osascript save image failed: %w", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return ErrNoImage
	}
	return nil
}

func pasteImageLinux(path string) error {
	var lastErr error
	if _, err := exec.LookPath("wl-paste"); err == nil {
		cmd := exec.Command("wl-paste", "--type", "image/png")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return os.WriteFile(path, out, 0600)
		}
		lastErr = err
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return os.WriteFile(path, out, 0600)
		}
		lastErr = err
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("neither wl-paste nor xclip found; install one to paste images on Linux")
}

func pasteImageWindows(path string) error {
	ps := psLoadWinForms + psLoadDrawing +
		`$img = [System.Windows.Forms.Clipboard]::GetImage(); ` +
		`if ($img -eq $null) { exit 1 }; ` +
		`$img.Save('` + strings.ReplaceAll(path, `'`, `''`) + `', [System.Drawing.Imaging.ImageFormat]::Png); ` +
		`$img.Dispose()`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("powershell clipboard image: %w", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return ErrNoImage
	}
	return nil
}
