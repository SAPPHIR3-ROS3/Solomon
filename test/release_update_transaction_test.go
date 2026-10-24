package test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestDesktopRestartArgsPreserveEmbeddedDesktopMode(t *testing.T) {
	for _, name := range []string{"solomon", "solomon.exe", "solomon-standalone", "SOLOMON-STANDALONE.EXE", "solomon-desktop", "solomon-desktop.exe"} {
		args := updater.DesktopRestartArgs(filepath.Join(t.TempDir(), name))
		var want []string
		if !strings.HasPrefix(strings.ToLower(name), "solomon-desktop") {
			want = []string{"desktop"}
		}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("restart %s: %v, want %v", name, args, want)
		}
	}
}

func TestReleaseUpdateCommitsCLIAndDesktopTogether(t *testing.T) {
	for _, scenario := range []string{"success", "no Go", "network", "CLI checksum", "missing desktop", "desktop checksum", "registration", "startup", "cancelled", "standalone"} {
		t.Run(scenario, func(t *testing.T) {
			cli, desktopPayload, _, _ := desktopInstallFixture(t)
			t.Setenv("SOLOMON_HOME", t.TempDir())
			t.Setenv("SOLOMON_BINARY", cli)
			if scenario == "no Go" {
				t.Setenv("PATH", t.TempDir())
			}
			if scenario == "standalone" {
				if err := os.WriteFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"), []byte("install_layout = 'standalone'\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			const tag = "v2099.1.0"
			desktop, err := updater.DesktopExecutablePath(cli)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(desktop), 0755); err != nil {
				t.Fatal(err)
			}
			originals := map[string][]byte{cli: []byte("original CLI"), desktop: []byte("old desktop"), desktop + ".install.json": []byte("old marker")}
			for path, data := range originals {
				if err := os.WriteFile(path, data, 0700); err != nil {
					t.Fatal(err)
				}
			}
			cliAsset, err := updater.ReleaseAssetName(tag)
			if err != nil {
				t.Fatal(err)
			}
			newCLI := []byte("new CLI")
			desktopDownloads, checksumRequests := 0, 0
			restoreHTTP := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
				if scenario == "network" {
					return nil, errors.New("download unavailable")
				}
				data, status := newCLI, http.StatusOK
				switch {
				case strings.HasSuffix(url, "/checksums.txt"):
					checksumRequests++
					// The desktop name is independent of the selected CLI layout.
					asset, err := updater.DesktopAssetName(tag, runtime.GOOS, runtime.GOARCH)
					if err != nil {
						t.Fatal(err)
					}
					hash := sha256.Sum256(desktopPayload)
					if scenario == "desktop checksum" {
						hash = sha256.Sum256([]byte("corrupt"))
					}
					cliHash := sha256.Sum256(newCLI)
					if scenario == "CLI checksum" {
						cliHash = sha256.Sum256([]byte("corrupt"))
					}
					data = []byte(fmt.Sprintf("%x  %s\n%x  %s\n", cliHash, cliAsset, hash, asset))
				case !strings.HasSuffix(url, "/"+cliAsset):
					desktopDownloads++
					data = desktopPayload
					if scenario == "missing desktop" {
						status = http.StatusNotFound
					}
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})
			defer restoreHTTP()
			restoreRegistrar := updater.SetDesktopRegistrar(func(context.Context, string, string, string, io.Writer) error {
				if scenario == "registration" {
					return errors.New("registration failed")
				}
				return nil
			})
			defer restoreRegistrar()
			assertOriginals := func() {
				t.Helper()
				for path, want := range originals {
					got, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("original %s changed: %q, %v", path, got, err)
					}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stops, starts, recoveries := 0, 0, 0
			err = lifecycle.ApplyReleaseUpdateForTest(ctx, tag, lifecycle.UpdateOpsForTest{
				Stop: func(string) error {
					stops++
					assertOriginals()
					if scenario != "standalone" && (desktopDownloads != 1 || checksumRequests != 2) {
						t.Fatal("interrupted runtime before both downloads were verified")
					}
					if scenario == "cancelled" {
						cancel()
						return ctx.Err()
					}
					return nil
				},
				Start: func(string) error {
					starts++
					got, _ := os.ReadFile(cli)
					if !bytes.Equal(got, newCLI) {
						t.Fatal("new CLI missing before restart")
					}
					if scenario != "standalone" {
						var marker struct{ Tag string }
						data, err := os.ReadFile(desktop + ".install.json")
						if err != nil || json.Unmarshal(data, &marker) != nil || marker.Tag != tag {
							t.Fatal("desktop not aligned with release before restart")
						}
					}
					if scenario == "startup" {
						return errors.New("new daemon failed")
					}
					return nil
				},
				Quiesce: func() error { return nil },
				Restore: func(string) error { recoveries++; assertOriginals(); return nil },
			})
			success := scenario == "success" || scenario == "no Go" || scenario == "standalone"
			if (err == nil) != success {
				t.Fatalf("unexpected result: %v", err)
			}
			if !success {
				assertOriginals()
			}
			if scenario == "missing desktop" || scenario == "desktop checksum" || scenario == "network" || scenario == "CLI checksum" {
				if stops != 0 || starts != 0 || recoveries != 0 {
					t.Fatal("failed preparation interrupted runtime")
				}
			}
			if scenario == "standalone" && desktopDownloads != 0 {
				t.Fatal("standalone release downloaded a split desktop")
			}
			for _, pattern := range []string{".solomon-update-*", ".solomon-desktop-update-*", ".solomon-rollback-*"} {
				for _, dir := range []string{filepath.Dir(cli), filepath.Dir(desktop)} {
					matches, _ := filepath.Glob(filepath.Join(dir, pattern))
					if len(matches) != 0 {
						t.Fatalf("leaked update files: %v", matches)
					}
				}
			}
		})
	}
}

func TestStandaloneUpgradeInfersLayoutAndHonorsExplicitSelection(t *testing.T) {
	for _, layout := range []string{"", "split", "standalone"} {
		t.Run("selection="+layout, func(t *testing.T) {
			t.Setenv("SOLOMON_HOME", t.TempDir())
			t.Setenv("SOLOMON_BINARY", filepath.Join(t.TempDir(), "solomon-standalone"))
			if layout != "" {
				if err := os.WriteFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"), []byte(fmt.Sprintf("install_layout = %q\n", layout)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			asset, err := updater.ReleaseAssetName("v2099.1.0")
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(asset, "solomon-standalone-") != (layout != "split") {
				t.Fatalf("incorrect asset for standalone: %s", asset)
			}
			executable, err := updater.DesktopUpdateExecutable(os.Getenv("SOLOMON_BINARY"))
			if err != nil || (executable == os.Getenv("SOLOMON_BINARY")) != (layout != "split") {
				t.Fatalf("incorrect desktop executable: %s, %v", executable, err)
			}
		})
	}
}

func TestLayoutSwitchUsesCoordinatedUpgrade(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v2099.1.0"}`)
	}))
	defer fixture.Close()
	restoreAPI := updater.SetLatestReleaseAPIURL(fixture.URL)
	defer restoreAPI()
	calls := 0
	failure := errors.New("fixture coordinator error")
	restoreExec := updater.SetExecInstallRestartHook(func(_ context.Context, tag string) error {
		calls++
		if tag != "v2099.1.0" {
			t.Fatalf("incorrect release: %s", tag)
		}
		return failure
	})
	defer restoreExec()
	for _, layout := range []string{"split", "standalone"} {
		if err := updater.SwitchToLayout(context.Background(), layout); !errors.Is(err, failure) {
			t.Fatalf("layout did not use coordinator: %v", err)
		}
	}
	if err := updater.SwitchToLayout(context.Background(), "invalid"); err == nil || calls != 2 {
		t.Fatalf("invalid layout dispatched: %v, calls=%d", err, calls)
	}
}
