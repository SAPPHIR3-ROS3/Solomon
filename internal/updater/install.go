package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func ReleaseAssetName(tag string) (string, error) {
	return releaseAssetName(tag)
}

func releaseAssetName(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("empty release tag")
	}
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}
	switch goos {
	case "linux", "darwin":
		return fmt.Sprintf("solomon-%s-%s-%s", tag, goos, goarch), nil
	case "windows":
		return fmt.Sprintf("solomon-%s-windows-%s.exe", tag, goarch), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s (use install.ps1 on Windows if needed)", goos)
	}
}

func releaseDownloadURL(tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", githubOwner, githubRepo, tag, asset)
}

func gopathBinDir() (string, error) {
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return filepath.Join(p, "bin"), nil
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin"), nil
	}
	return "", fmt.Errorf("could not resolve GOPATH/bin")
}

func installTargetPath() (string, error) {
	dir, err := gopathBinDir()
	if err != nil {
		return "", err
	}
	name := "solomon"
	if runtime.GOOS == "windows" {
		name = "solomon.exe"
	}
	return filepath.Join(dir, name), nil
}

var httpDownload = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "solomon-updater")
	client := &http.Client{Timeout: 10 * time.Minute}
	return client.Do(req)
}

func Install(ctx context.Context, tag string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	asset, err := releaseAssetName(tag)
	if err != nil {
		return err
	}
	target, err := installTargetPath()
	if err != nil {
		return err
	}
	url := releaseDownloadURL(tag, asset)
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Fprintf(progress, "Downloading %s...\n", asset)
	resp, err := httpDownload(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".solomon-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return err
		}
	}
	backup := target + ".bak"
	_ = os.Remove(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup current binary: %w", err)
		}
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("install binary: %w", err)
	}
	ok = true
	_ = os.Remove(backup)
	fmt.Fprintf(progress, "Installed %s to %s\n", tag, target)
	fmt.Fprintln(progress, "Restart Solomon to use the new version.")
	return nil
}

const (
	installScriptRawURL = "https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.sh"
	installPS1RawURL    = "https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.ps1"
)

func InstallCommand(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("empty release tag")
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		return fmt.Sprintf("SOLOMON_VERSION=%s curl -fsSL %s | bash", tag, installScriptRawURL), nil
	case "windows":
		escaped := strings.ReplaceAll(tag, "'", "''")
		return fmt.Sprintf("$env:SOLOMON_VERSION='%s'; irm %s | iex", escaped, installPS1RawURL), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func RunSystemInstall(ctx context.Context, tag string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("empty release tag")
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		script := fmt.Sprintf("SOLOMON_VERSION=%s curl -fsSL %s | bash", tag, installScriptRawURL)
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		cmd.Stdout = progress
		cmd.Stderr = progress
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(progress, "Install script failed (%v); trying direct download...\n", err)
			return Install(ctx, tag, progress)
		}
		fmt.Fprintln(progress, "Restart Solomon to use the new version.")
		return nil
	case "windows":
		ps := fmt.Sprintf("$env:SOLOMON_VERSION='%s'; irm %s | iex", strings.ReplaceAll(tag, "'", "''"), installPS1RawURL)
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
		cmd.Stdout = progress
		cmd.Stderr = progress
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(progress, "Install script failed (%v); trying direct download...\n", err)
			return Install(ctx, tag, progress)
		}
		fmt.Fprintln(progress, "Restart Solomon to use the new version.")
		return nil
	default:
		return Install(ctx, tag, progress)
	}
}
