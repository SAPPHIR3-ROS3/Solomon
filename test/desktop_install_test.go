package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestDesktopAssetsCoverAllPlatforms(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			asset, err := updater.DesktopAssetName("v2026.1005.0", goos, arch)
			if err != nil || !strings.Contains(asset, goos+"-"+arch) {
				t.Fatalf("asset %s/%s = %q, %v", goos, arch, asset, err)
			}
		}
	}
	if _, err := updater.DesktopAssetName("../../main", "windows", "amd64"); err == nil {
		t.Fatal("invalid release tag accepted")
	}
}

func desktopInstallFixture(t *testing.T) (string, []byte, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	cli := filepath.Join(dir, "custom CLI directory", "solomon")
	if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cli, []byte("original CLI"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("desktop executable")
	if runtime.GOOS == "darwin" {
		var archive bytes.Buffer
		gz := gzip.NewWriter(&archive)
		writer := tar.NewWriter(gz)
		if err := writer.WriteHeader(&tar.Header{Name: "Solomon.app/Contents/MacOS/solomon-desktop", Mode: 0o755, Size: int64(len(payload))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		payload = archive.Bytes()
	}
	downloads, registrations := &atomic.Int32{}, &atomic.Int32{}
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		downloads.Add(1)
		body := payload
		if strings.HasSuffix(url, "/checksums.txt") {
			parts := strings.Split(url, "/")
			tag := parts[len(parts)-2]
			asset, err := updater.DesktopAssetName(tag, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			body = []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), asset))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	t.Cleanup(restore)
	restoreRegistration := updater.SetDesktopRegistrar(func(_ context.Context, tag, sourceCLI, target string, _ io.Writer) error {
		registrations.Add(1)
		if sourceCLI != cli {
			t.Errorf("registered CLI = %s, want %s", sourceCLI, cli)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("GUI missing before registration: %v", err)
		}
		return nil
	})
	t.Cleanup(restoreRegistration)
	return cli, payload, downloads, registrations
}

func TestDesktopFirstLaunchInstallsOnceAndUpdatesWithCLI(t *testing.T) {
	cli, _, downloads, registrations := desktopInstallFixture(t)
	for _, tag := range []string{"v2026.1005.0", "v2026.1005.0", "v2026.1005.1"} {
		if _, err := updater.EnsureDesktop(context.Background(), tag, cli, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if downloads.Load() != 4 || registrations.Load() != 2 {
		t.Fatalf("downloads=%d registrations=%d; repeated startup should be offline", downloads.Load(), registrations.Load())
	}
	data, err := os.ReadFile(cli)
	if err != nil || string(data) != "original CLI" {
		t.Fatal("desktop setup changed the CLI")
	}
}

func TestDesktopDevelopmentFirstLaunchResolvesLatestRelease(t *testing.T) {
	cli, _, downloads, _ := desktopInstallFixture(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v2026.1005.0"}`) }))
	defer api.Close()
	restore := updater.SetLatestReleaseAPIURL(api.URL)
	defer restore()
	if _, err := updater.EnsureDesktop(context.Background(), "dev", cli, io.Discard); err != nil {
		t.Fatal(err)
	}
	api.Close()
	if _, err := updater.EnsureDesktop(context.Background(), "dev", cli, io.Discard); err != nil {
		t.Fatalf("second startup needs no network: %v", err)
	}
	if downloads.Load() != 2 {
		t.Fatalf("unexpected downloads: %d", downloads.Load())
	}
}

func TestDesktopConcurrentFirstLaunchDownloadsOnce(t *testing.T) {
	cli, _, downloads, registrations := desktopInstallFixture(t)
	var group sync.WaitGroup
	errors := make(chan error, 4)
	for attempt := 0; attempt < 4; attempt++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := updater.EnsureDesktop(context.Background(), "v2026.1005.0", cli, io.Discard)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if downloads.Load() != 2 || registrations.Load() != 1 {
		t.Fatalf("concurrent setup: downloads=%d registrations=%d", downloads.Load(), registrations.Load())
	}
}

func TestDesktopRegistrationFailureRestoresInstalledGUIAndMarker(t *testing.T) {
	cli, _, _, _ := desktopInstallFixture(t)
	target, err := updater.EnsureDesktop(context.Background(), "v2026.1005.0", cli, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(target + ".install.json")
	if err != nil {
		t.Fatal(err)
	}
	restore := updater.SetDesktopRegistrar(func(context.Context, string, string, string, io.Writer) error {
		return errors.New("application registration failed")
	})
	defer restore()
	if err := updater.InstallDesktop(context.Background(), "v2026.1005.1", cli, io.Discard); err == nil {
		t.Fatal("registration failure reported success")
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("registration failure lost previous GUI: %v", err)
	}
	afterMarker, err := os.ReadFile(target + ".install.json")
	if err != nil || !bytes.Equal(marker, afterMarker) {
		t.Fatalf("registration failure lost previous marker: %v", err)
	}
}

func TestDesktopChecksumFailurePreservesInstalledGUI(t *testing.T) {
	cli, _, _, registrations := desktopInstallFixture(t)
	if _, err := updater.EnsureDesktop(context.Background(), "v2026.1005.0", cli, io.Discard); err != nil {
		t.Fatal(err)
	}
	target, err := updater.DesktopExecutablePath(cli)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		body := "corrupt executable"
		if strings.HasSuffix(url, "/checksums.txt") {
			asset, _ := updater.DesktopAssetName("v2026.1005.1", runtime.GOOS, runtime.GOARCH)
			body = strings.Repeat("0", 64) + "  " + asset
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()
	_, err = updater.EnsureDesktop(context.Background(), "v2026.1005.1", cli, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum error: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) || registrations.Load() != 1 {
		t.Fatal("failed update changed the installed GUI or registration")
	}
}

func TestDesktopMissingChecksumsDoesNotRegister(t *testing.T) {
	cli, payload, _, registrations := desktopInstallFixture(t)
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		status := http.StatusOK
		if strings.HasSuffix(url, "/checksums.txt") {
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})
	defer restore()
	if err := updater.InstallDesktop(context.Background(), "v2026.1005.0", cli, io.Discard); err == nil || !strings.Contains(err.Error(), "required desktop checksums") {
		t.Fatalf("missing checksums error: %v", err)
	}
	if registrations.Load() != 0 {
		t.Fatal("unverified GUI was registered")
	}
}

func TestDesktopEmptyDownloadPreservesExistingInstallation(t *testing.T) {
	cli, _, _, registrations := desktopInstallFixture(t)
	target, err := updater.EnsureDesktop(context.Background(), "v2026.1005.0", cli, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(target)
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		body := ""
		if strings.HasSuffix(url, "/checksums.txt") {
			asset, _ := updater.DesktopAssetName("v2026.1005.1", runtime.GOOS, runtime.GOARCH)
			body = fmt.Sprintf("%x  %s\n", sha256.Sum256(nil), asset)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()
	if err := updater.InstallDesktop(context.Background(), "v2026.1005.1", cli, io.Discard); err == nil {
		t.Fatal("empty desktop download accepted")
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) || registrations.Load() != 1 {
		t.Fatalf("empty download changed installed desktop: %v", err)
	}
}

func TestBuiltDesktopRejectsIncompleteArtifactsBeforeReplacement(t *testing.T) {
	cli, _, _, registrations := desktopInstallFixture(t)
	target, err := updater.EnsureDesktop(context.Background(), "v2026.1005.0", cli, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(target)
	for _, kind := range []string{"missing", "directory", "empty", "not executable"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "not executable" && runtime.GOOS == "windows" {
				t.Skip("Windows executable files do not use Unix permission bits")
			}
			source := filepath.Join(t.TempDir(), "built desktop")
			executable := source
			if runtime.GOOS == "darwin" {
				executable = filepath.Join(source, "Contents", "MacOS", "solomon-desktop")
				if err := os.MkdirAll(filepath.Dir(executable), 0755); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "directory":
				err = os.Mkdir(executable, 0755)
			case "empty":
				err = os.WriteFile(executable, nil, 0755)
			case "not executable":
				err = os.WriteFile(executable, []byte("desktop"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := updater.InstallBuiltDesktop(context.Background(), "dev", cli, source, io.Discard); err == nil {
				t.Fatal("incomplete built desktop accepted")
			}
			after, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(before, after) || registrations.Load() != 1 {
				t.Fatalf("invalid build replaced existing desktop: %v", err)
			}
		})
	}
}
