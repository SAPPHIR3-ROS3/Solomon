package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

var desktopReleaseTag = regexp.MustCompile(`^v[0-9]{4}\.[0-9]+\.[0-9]+$`)

type desktopInstallation struct {
	Tag string `json:"tag"`
	CLI string `json:"cli"`
}

var desktopRegistrar = registerDesktop

// SetDesktopRegistrar replaces OS registration for isolated installer tests.
func SetDesktopRegistrar(fn func(context.Context, string, string, string, io.Writer) error) func() {
	previous := desktopRegistrar
	desktopRegistrar = fn
	return func() { desktopRegistrar = previous }
}

func DesktopAssetName(tag, goos, arch string) (string, error) {
	if !desktopReleaseTag.MatchString(tag) || (arch != "amd64" && arch != "arm64") {
		return "", fmt.Errorf("unsupported desktop release or architecture: %s/%s", tag, arch)
	}
	name := fmt.Sprintf("solomon-desktop-%s-%s-%s", tag, goos, arch)
	switch goos {
	case "windows":
		return name + ".exe", nil
	case "darwin":
		return name + ".app.tar.gz", nil
	case "linux":
		return name, nil
	default:
		return "", fmt.Errorf("unsupported desktop OS: %s", goos)
	}
}

func DesktopExecutablePath(cli string) (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Applications", "Solomon.app", "Contents", "MacOS", "solomon-desktop"), nil
	}
	name := "solomon-desktop"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(cli), name), nil
}

// EnsureDesktop completes a CLI-only installation on its first normal launch.
// Subsequent launches use the recorded version without consulting the network.
func EnsureDesktop(ctx context.Context, version, cli string, progress io.Writer) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cli, err := filepath.Abs(cli)
	if err != nil {
		return "", err
	}
	target, err := DesktopExecutablePath(cli)
	if err != nil {
		return "", err
	}
	lock, err := lockDesktopInstallation(ctx, target)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	tag := strings.TrimSpace(version)
	release := desktopReleaseTag.MatchString(tag)
	info, statErr := os.Stat(target)
	installed := statErr == nil && !info.IsDir()
	var recorded desktopInstallation
	if data, err := os.ReadFile(target + ".install.json"); err == nil {
		_ = json.Unmarshal(data, &recorded)
	}
	if installed && recorded.CLI == cli && ((!release && recorded.Tag != "") || recorded.Tag == tag) {
		return target, nil
	}
	if installed && (!release || recorded.Tag == tag) {
		if err := finishDesktopInstallation(ctx, tag, cli, target, progress); err != nil {
			return "", err
		}
		return target, nil
	}
	if !release {
		result := Check(ctx, "dev")
		if result.Err != nil {
			return "", result.Err
		}
		tag = result.LatestTag
	}
	if err := installDesktop(ctx, tag, cli, progress); err != nil {
		return "", err
	}
	return target, nil
}

func InstallDesktop(ctx context.Context, tag, cli string, progress io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cli, err := filepath.Abs(cli)
	if err != nil {
		return err
	}
	target, err := DesktopExecutablePath(cli)
	if err != nil {
		return err
	}
	lock, err := lockDesktopInstallation(ctx, target)
	if err != nil {
		return err
	}
	defer lock.Close()
	return installDesktop(ctx, tag, cli, progress)
}

// InstallBuiltDesktop deploys the already built native client without a release
// lookup. On macOS source is the complete Solomon.app bundle.
func InstallBuiltDesktop(ctx context.Context, version, cli, source string, progress io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cli, err := filepath.Abs(cli)
	if err != nil {
		return err
	}
	target, err := DesktopExecutablePath(cli)
	if err != nil {
		return err
	}
	lock, err := lockDesktopInstallation(ctx, target)
	if err != nil {
		return err
	}
	defer lock.Close()
	destination := target
	executable := source
	if runtime.GOOS == "darwin" {
		destination = filepath.Dir(filepath.Dir(filepath.Dir(target)))
		executable = filepath.Join(source, "Contents", "MacOS", "solomon-desktop")
	}
	if err := validateDesktopExecutable(executable); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".solomon-built-desktop-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, filepath.Base(destination))
	if err := copyInstallationPath(source, staged); err != nil {
		return err
	}
	if err := replaceInstallationPath(staged, destination); err != nil {
		return err
	}
	return finishDesktopInstallation(ctx, version, cli, target, progress)
}

func lockDesktopInstallation(ctx context.Context, target string) (*flock.Flock, error) {
	path := target + ".install.lock"
	if runtime.GOOS == "darwin" {
		bundle := filepath.Dir(filepath.Dir(filepath.Dir(target)))
		// The lock must survive replacing the whole .app directory.
		path = filepath.Join(filepath.Dir(bundle), ".solomon-desktop.install.lock")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	lock := flock.New(path)
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		lock.Close()
		return nil, err
	}
	if !locked {
		lock.Close()
		return nil, ctx.Err()
	}
	return lock, nil
}

func installDesktop(ctx context.Context, tag, cli string, progress io.Writer) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if progress == nil {
		progress = io.Discard
	}
	asset, err := DesktopAssetName(tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	cli, err = filepath.Abs(cli)
	if err != nil {
		return err
	}
	if info, err := os.Stat(cli); err != nil || info.IsDir() {
		return fmt.Errorf("desktop installation requires an installed CLI at %s", cli)
	}
	target, err := DesktopExecutablePath(cli)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	fmt.Fprintf(progress, "Installing Solomon Desktop (%s)...\n", tag)
	response, err := httpDownload(ctx, releaseDownloadURL(tag, asset))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", asset, response.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".solomon-desktop-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, response.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if info, err := os.Stat(tmp.Name()); err != nil || info.Size() == 0 {
		return fmt.Errorf("downloaded desktop is empty or unreadable: %s", asset)
	}
	if err := verifyAssetChecksum(ctx, tag, asset, tmp.Name(), progress, true); err != nil {
		return err
	}
	backupPaths := []string{target, target + ".install.json"}
	if runtime.GOOS == "darwin" {
		backupPaths = []string{filepath.Dir(filepath.Dir(filepath.Dir(target)))}
	}
	backup, err := BackupInstallation(backupPaths...)
	if err != nil {
		return err
	}
	defer backup.Close()
	defer func() {
		if err != nil {
			err = errors.Join(err, backup.Restore())
		}
	}()
	if runtime.GOOS == "darwin" {
		if err := installDesktopBundle(tmp.Name(), target); err != nil {
			return err
		}
	} else {
		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return err
		}
		if err := replaceDesktopPath(tmp.Name(), target); err != nil {
			return err
		}
	}
	return finishDesktopInstallation(ctx, tag, cli, target, progress)
}

func finishDesktopInstallation(ctx context.Context, tag, cli, target string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	if err := desktopRegistrar(ctx, tag, cli, target, progress); err != nil {
		return err
	}
	data, err := json.Marshal(desktopInstallation{Tag: tag, CLI: cli})
	if err != nil {
		return err
	}
	return os.WriteFile(target+".install.json", data, 0o600)
}

func replaceDesktopPath(source, target string) error {
	return replaceInstallationPath(source, target)
}

func validateDesktopExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("desktop requires a nonempty executable: %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("desktop is not executable: %s", path)
	}
	return nil
}

func installDesktopBundle(archive, executable string) error {
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
	stage, err := os.MkdirTemp(filepath.Dir(bundle), ".solomon-app-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name != "Solomon.app" && !strings.HasPrefix(name, "Solomon.app"+string(filepath.Separator)) {
			return fmt.Errorf("invalid desktop archive path: %s", header.Name)
		}
		path := filepath.Join(stage, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, reader)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported desktop archive entry: %s", header.Name)
		}
	}
	if err := validateDesktopExecutable(filepath.Join(stage, "Solomon.app", "Contents", "MacOS", "solomon-desktop")); err != nil {
		return err
	}
	return replaceDesktopPath(filepath.Join(stage, "Solomon.app"), bundle)
}
