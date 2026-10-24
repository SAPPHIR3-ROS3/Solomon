package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// PreparedDesktop keeps a verified release off the live installation until the
// runtime coordinator has stopped clients and backed up both CLI and desktop.
type PreparedDesktop struct {
	directory, staged, target, executable, cli, tag string
}

func PrepareDesktopUpdate(ctx context.Context, tag, cli string, progress io.Writer) (*PreparedDesktop, error) {
	if selectedInstallLayout() == "standalone" {
		return nil, nil
	}
	if progress == nil {
		progress = io.Discard
	}
	asset, err := DesktopAssetName(tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	target, err := DesktopExecutablePath(cli)
	if err != nil {
		return nil, err
	}
	destination := target
	if runtime.GOOS == "darwin" {
		destination = filepath.Dir(filepath.Dir(filepath.Dir(target)))
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp(filepath.Dir(destination), ".solomon-desktop-update-*")
	if err != nil {
		return nil, err
	}
	p := &PreparedDesktop{directory: directory, target: destination, executable: target, cli: cli, tag: tag}
	ok := false
	defer func() {
		if !ok {
			p.Close()
		}
	}()
	fmt.Fprintf(progress, "Downloading %s...\n", asset)
	response, err := httpDownload(ctx, releaseDownloadURL(tag, asset))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", asset, response.Status)
	}
	download := filepath.Join(directory, "download")
	out, err := os.OpenFile(download, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(out, trackDownload(response.Body, response.ContentLength, progress))
	closeErr := out.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(download, 0755); err != nil {
			return nil, err
		}
	}
	if err := validateDesktopExecutable(download); err != nil {
		return nil, err
	}
	if err := verifyAssetChecksum(ctx, tag, asset, download, progress, true); err != nil {
		return nil, err
	}
	p.staged = download
	if runtime.GOOS == "darwin" {
		p.staged = filepath.Join(directory, "Solomon.app")
		if err := installDesktopBundle(download, filepath.Join(p.staged, "Contents", "MacOS", "solomon-desktop")); err != nil {
			return nil, err
		}
	}
	ok = true
	return p, nil
}

func (p *PreparedDesktop) BackupPaths() []string {
	if p == nil {
		return nil
	}
	if runtime.GOOS == "darwin" {
		return []string{p.target}
	}
	return []string{p.target, p.executable + ".install.json"}
}

// Commit participates in the coordinator's transaction, which owns rollback.
func (p *PreparedDesktop) Commit(ctx context.Context, progress io.Writer) error {
	if p == nil {
		return nil
	}
	lock, err := lockDesktopInstallation(ctx, p.executable)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := replaceInstallationPath(p.staged, p.target); err != nil {
		return err
	}
	return finishDesktopInstallation(ctx, p.tag, p.cli, p.executable, progress)
}

func (p *PreparedDesktop) Close() {
	if p != nil {
		_ = os.RemoveAll(p.directory)
	}
}
