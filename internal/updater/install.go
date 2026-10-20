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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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
	name := "solomon"
	if selectedInstallLayout() == "standalone" {
		name = "solomon-standalone"
	}
	switch goos {
	case "linux", "darwin":
		return fmt.Sprintf("%s-%s-%s-%s", name, tag, goos, goarch), nil
	case "windows":
		return fmt.Sprintf("%s-%s-windows-%s.exe", name, tag, goarch), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s (use install.ps1 on Windows if needed)", goos)
	}
}

func releaseDownloadURL(tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", githubOwner, githubRepo, tag, asset)
}

func goInstallBinDir() (string, error) {
	if out, err := exec.Command("go", "env", "GOBIN").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p, nil
		}
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return filepath.Join(filepath.SplitList(p)[0], "bin"), nil
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin"), nil
	}
	return "", fmt.Errorf("could not resolve Go install bin directory")
}

func selectedInstallLayout() string {
	root, err := config.Load()
	if err != nil || root == nil {
		return "split"
	}
	return root.EffectiveInstallLayout()
}

func installTargetPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SOLOMON_BINARY")); configured != "" {
		return filepath.Abs(configured)
	}
	if executable, err := os.Executable(); err == nil {
		base := strings.ToLower(filepath.Base(executable))
		if selectedInstallLayout() == "standalone" {
			if base == "solomon-standalone" || base == "solomon-standalone.exe" {
				return executable, nil
			}
		} else if base == "solomon" || base == "solomon.exe" {
			return executable, nil
		}
		if base == "solomon" || base == "solomon.exe" || base == "solomon-standalone" || base == "solomon-standalone.exe" {
			return layoutInstallPath(filepath.Dir(executable)), nil
		}
	}
	dir, err := goInstallBinDir()
	if err != nil {
		return "", err
	}
	return layoutInstallPath(dir), nil
}

func layoutInstallPath(dir string) string {
	name := "solomon"
	if selectedInstallLayout() == "standalone" {
		name = "solomon-standalone"
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

var httpDownload = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "solomon-updater")
	if tok := githubAuthToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	return client.Do(req)
}

func SetHTTPDownload(fn func(context.Context, string) (*http.Response, error)) func() {
	prev := httpDownload
	if fn == nil {
		httpDownload = func(ctx context.Context, url string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", "solomon-updater")
			if tok := githubAuthToken(); tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
			client := &http.Client{Timeout: 10 * time.Minute}
			return client.Do(req)
		}
	} else {
		httpDownload = fn
	}
	return func() { httpDownload = prev }
}

// PrepareInstall downloads and verifies a release beside its final destination.
// It does not replace the running installation; the caller owns the staged file.
func PrepareInstall(ctx context.Context, tag string, progress io.Writer) (staged, target string, err error) {
	if progress == nil {
		progress = io.Discard
	}
	asset, err := releaseAssetName(tag)
	if err != nil {
		return "", "", err
	}
	target, err = installTargetPath()
	if err != nil {
		return "", "", err
	}
	url := releaseDownloadURL(tag, asset)
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Fprintf(progress, "Downloading %s...\n", asset)
	resp, err := httpDownload(ctx, url)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "updater download failed", logging.LogOptions{Params: map[string]any{"tag": tag, "url": url, "err": err.Error()}})
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".solomon-update-*")
	if err != nil {
		return "", "", err
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
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		return "", "", err
	}
	if err := verifyReleaseAsset(ctx, tag, asset, tmpPath, progress); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "updater verify release failed", logging.LogOptions{Params: map[string]any{"tag": tag, "asset": asset, "err": err.Error()}})
		return "", "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return "", "", err
		}
	}
	ok = true
	return tmpPath, target, nil
}

// VerifyPreparedInstall checks the handoff again before stopping the runtime.
func VerifyPreparedInstall(ctx context.Context, tag, staged, target string) error {
	expectedTarget, err := installTargetPath()
	if err != nil {
		return err
	}
	info, err := os.Lstat(staged)
	if err != nil {
		return err
	}
	if target != expectedTarget || !info.Mode().IsRegular() || info.Size() == 0 || filepath.Dir(staged) != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(staged), ".solomon-update-") {
		return fmt.Errorf("invalid prepared update")
	}
	asset, err := releaseAssetName(tag)
	if err != nil {
		return err
	}
	return verifyReleaseAsset(ctx, tag, asset, staged, io.Discard)
}

func Install(ctx context.Context, tag string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	staged, target, err := PrepareInstall(ctx, tag, progress)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	if err := CommitInstall(staged, target); err != nil {
		return err
	}
	logging.Log(logging.INFO_LOG_LEVEL, "updater install complete", logging.LogOptions{Params: map[string]any{"tag": tag, "path": target}})
	fmt.Fprintf(progress, "Installed %s to %s\n", tag, target)
	return nil
}

// CommitInstall replaces the binary atomically, restoring it on failure.
func CommitInstall(staged, target string) error {
	info, err := os.Lstat(staged)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("staged binary must be a nonempty regular file")
	}
	return replaceInstallationPath(staged, target)
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
		return fmt.Sprintf("curl -fsSL %s | SOLOMON_VERSION='%s' bash", installScriptRawURL, strings.ReplaceAll(tag, "'", "'\"'\"'")), nil
	case "windows":
		escaped := strings.ReplaceAll(tag, "'", "''")
		return fmt.Sprintf("$env:SOLOMON_VERSION='%s'; irm %s | iex", escaped, installPS1RawURL), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func InstallFallbackMessage(tag string) string {
	tag = strings.TrimSpace(tag)
	cmd, err := InstallCommand(tag)
	if err != nil {
		return "If automatic upgrade fails, retry with your platform install command."
	}
	return fmt.Sprintf("If automatic upgrade fails, retry with your platform install command:\n%s", cmd)
}

func canExecInstallRestart() bool {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
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
	if runtime.GOOS == "windows" {
		if err := runWindowsProfileSetup(ctx, progress); err != nil {
			msg := fmt.Sprintf("Warning: PowerShell profile update failed: %v\n", err)
			if progress != io.Discard {
				fmt.Fprint(progress, msg)
			} else {
				fmt.Fprint(os.Stderr, msg)
			}
		}
	}
	if canExecInstallRestart() {
		return ErrRestartScheduled
	}
	if err := scheduleInstallRestart(ctx, tag, progress); err == nil {
		return ErrRestartScheduled
	} else if progress != nil {
		fmt.Fprintf(progress, "Detached install failed (%v); trying in-process download...\n", err)
	}
	if err := Install(ctx, tag, progress); err != nil {
		return err
	}
	exe, err := restartExecutable()
	if err != nil {
		fmt.Fprintln(progress, "Restart Solomon manually to use the new version.")
		return ErrRestartScheduled
	}
	cwd, _ := os.Getwd()
	if err := scheduleRestartOnly(ctx, os.Getpid(), cwd, exe, os.Args[1:], progress); err != nil {
		fmt.Fprintln(progress, "Restart Solomon manually to use the new version.")
		return ErrRestartScheduled
	}
	return ErrRestartScheduled
}

// SwitchToLayout downloads the saved layout from the latest release and installs it.
func SwitchToLayout(ctx context.Context, layout string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	result := Check(ctx, "dev")
	if result.Err != nil {
		return result.Err
	}
	tag := strings.TrimSpace(result.LatestTag)
	if tag == "" {
		return fmt.Errorf("no release available to switch layout")
	}
	if err := Install(ctx, tag, io.Discard); err != nil {
		return err
	}
	if layout != "split" {
		return nil
	}
	cli, err := installTargetPath()
	if err != nil {
		return err
	}
	return InstallDesktop(ctx, tag, cli, io.Discard)
}
