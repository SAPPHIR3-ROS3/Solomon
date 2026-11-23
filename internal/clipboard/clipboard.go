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

var ErrNoImage = errors.New("no supported image in the clipboard: copy the image itself as PNG, JPEG, WebP, or GIF, or attach it by file path")
var ErrUnavailable = errors.New("clipboard image access unavailable")
var ErrImageTooLarge = errors.New("clipboard image exceeds 32 MiB or 32 mebipixels: resize it or attach it by file path")

const (
	psLoadWinForms = `Add-Type -AssemblyName System.Windows.Forms; `
	psLoadDrawing  = `Add-Type -AssemblyName System.Drawing; `
)

func HasImage() bool {
	return ImageStatus() == nil
}

func ImageStatus() error {
	switch runtime.GOOS {
	case "darwin":
		return imageStatusDarwin()
	case "linux":
		return linuxImageStatus()
	case "windows":
		return imageStatusWindows()
	default:
		return fmt.Errorf("%w on %s: attach the image by file path", ErrUnavailable, runtime.GOOS)
	}
}

func imageStatusDarwin() error {
	info, err := exec.Command("osascript", "-e", "clipboard info").Output()
	if err != nil {
		return fmt.Errorf("%w: macOS could not inspect the pasteboard (%v). Run Solomon from your desktop session or attach the image by file path", ErrUnavailable, err)
	}
	s := string(info)
	if !strings.Contains(s, "PNGf") && !strings.Contains(s, "JPEG") && !strings.Contains(s, "TIFF") {
		return ErrNoImage
	}
	return nil
}

func imageStatusWindows() error {
	ps := psLoadWinForms + `$ErrorActionPreference = 'Stop'; [System.Windows.Forms.Clipboard]::ContainsImage()`
	data, err := clipboardPowerShell(ps).Output()
	if err != nil {
		return fmt.Errorf("%w: Windows could not inspect the clipboard (%v). Run Solomon from your desktop session or attach the image by file path", ErrUnavailable, err)
	}
	switch strings.TrimSpace(string(data)) {
	case "True":
		return nil
	case "False":
		return ErrNoImage
	default:
		return fmt.Errorf("%w: unexpected Windows clipboard response. Retry or attach the image by file path", ErrUnavailable)
	}
}

func PasteImage(dir, chatID string, seq int) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create attachment directory: %w. Check its permissions and available disk space", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.%d.png", chatID, seq))
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = pasteImageDarwin(path)
	case "linux":
		err = pasteImageLinux(path)
	case "windows":
		err = pasteImageWindows(path)
	default:
		err = fmt.Errorf("%w on %s: attach the image by file path", ErrUnavailable, runtime.GOOS)
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func pasteImageDarwin(path string) error {
	script := fmt.Sprintf(`
set imgData to the clipboard as «class PNGf»
set outFile to open for access POSIX file %q with write permission
write imgData to outFile
close access outFile
`, path)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("macOS could not save the clipboard image: %w. Copy the image as PNG and check attachment directory permissions", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return ErrNoImage
	}
	return nil
}

func pasteImageWindows(path string) error {
	ps := psLoadWinForms + psLoadDrawing +
		`$img = [System.Windows.Forms.Clipboard]::GetImage(); ` +
		`if ($img -eq $null) { exit 1 }; ` +
		`$img.Save('` + strings.ReplaceAll(path, `'`, `''`) + `', [System.Drawing.Imaging.ImageFormat]::Png); ` +
		`$img.Dispose()`
	cmd := clipboardPowerShell(ps)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Windows could not save the clipboard image: %w. Copy the image again and check attachment directory permissions", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return ErrNoImage
	}
	return nil
}
