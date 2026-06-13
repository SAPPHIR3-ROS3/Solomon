package test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestIsNewerRelease_calendarSemver(t *testing.T) {
	if !updater.IsNewerRelease("v2026.602.1", "v2026.527.2") {
		t.Fatal("expected newer")
	}
	if updater.IsNewerRelease("v2026.527.2", "v2026.602.1") {
		t.Fatal("expected not newer")
	}
	if !updater.IsNewerRelease("v2026.602.1", "dev") {
		t.Fatal("dev should be older than release")
	}
	if updater.IsNewerRelease("v2026.527.2", "v2026.527.2") {
		t.Fatal("same tag should not be newer")
	}
}

func TestCheck_githubLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2099.101.0"})
	}))
	defer srv.Close()

	restore := updater.SetLatestReleaseAPIURL(srv.URL)
	defer restore()

	res := updater.Check(context.Background(), "v2026.101.0")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if !res.Newer || res.LatestTag != "v2099.101.0" {
		t.Fatalf("got %+v", res)
	}
	if n := res.Notice(); n == nil || n.Latest != "v2099.101.0" {
		t.Fatalf("notice %+v", n)
	}
}

func TestReleaseAssetName(t *testing.T) {
	name, err := updater.ReleaseAssetName("v2026.101.0")
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("empty asset")
	}
}

func TestInstallCommand(t *testing.T) {
	t.Parallel()
	cmd, err := updater.InstallCommand("v2026.602.1")
	if err != nil {
		t.Fatal(err)
	}
	if cmd == "" {
		t.Fatal("empty command")
	}
	if !strings.Contains(cmd, "v2026.602.1") {
		t.Fatalf("tag missing from %q", cmd)
	}
}

func TestInstall_verifiesChecksum(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install target path layout")
	}
	payload := []byte("solomon-binary-payload")
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	tag := "v2099.101.0"
	asset, err := updater.ReleaseAssetName(tag)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n", hash, asset)
		case strings.HasSuffix(r.URL.Path, "/"+asset):
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreHTTP := updater.SetHTTPDownload(func(ctx context.Context, url string) (*http.Response, error) {
		base := "https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/" + tag + "/"
		path := strings.TrimPrefix(url, base)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/"+path, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req)
	})
	defer restoreHTTP()

	t.Setenv("GOPATH", t.TempDir())

	if err := updater.Install(context.Background(), tag, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestInstall_checksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install target path layout")
	}
	payload := []byte("solomon-binary-payload")
	tag := "v2099.101.0"
	asset, err := updater.ReleaseAssetName(tag)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "deadbeef  %s\n", asset)
		case strings.HasSuffix(r.URL.Path, "/"+asset):
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreHTTP := updater.SetHTTPDownload(func(ctx context.Context, url string) (*http.Response, error) {
		base := "https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/" + tag + "/"
		path := strings.TrimPrefix(url, base)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/"+path, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req)
	})
	defer restoreHTTP()

	t.Setenv("GOPATH", t.TempDir())

	err = updater.Install(context.Background(), tag, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}
