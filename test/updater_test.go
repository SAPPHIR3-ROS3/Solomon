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
	"time"

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
		t.Fatal("dev should be considered behind a published release")
	}
	if updater.IsNewerRelease("v2026.527.2", "v2026.527.2") {
		t.Fatal("same tag should not be newer")
	}
}

func TestIsDevelopmentVersion(t *testing.T) {
	t.Parallel()
	for _, version := range []string{
		"",
		"dev",
		"dev-b50aa3c-dirty",
		"v2026.909.1-dev",
		"v2026.909.1-dev-f8bf95c-dirty",
		"v2026.613.1-0.20260908173929-6c06527f7272",
		"v0.0.0-20260908173929-6c06527f7272",
	} {
		if !updater.IsDevelopmentVersion(version) {
			t.Errorf("IsDevelopmentVersion(%q) = false", version)
		}
	}
	for _, version := range []string{"v2026.909.1", "v2026.909.1+build"} {
		if updater.IsDevelopmentVersion(version) {
			t.Errorf("IsDevelopmentVersion(%q) = true", version)
		}
	}
}

func TestCheckWithCommit_doesNotUpdateAheadDevelopmentBuild(t *testing.T) {
	latest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2026.909.1"})
	}))
	defer latest.Close()
	compare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "v2026.909.1...") {
			t.Fatalf("unexpected compare path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ahead"})
	}))
	defer compare.Close()

	restoreLatest := updater.SetLatestReleaseAPIURL(latest.URL)
	defer restoreLatest()
	restoreCompare := updater.SetCompareReleaseAPIURL(compare.URL)
	defer restoreCompare()

	res := updater.CheckWithCommit(context.Background(), "v2026.716.0-dev-f8bf95c", strings.Repeat("a", 40))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Newer || res.Notice() != nil || res.LocalCommitRelation != "ahead" {
		t.Fatalf("ahead development build should stay untouched: %+v", res)
	}
}

func TestCheckWithCommit_doesUpdateBehindDevelopmentBuild(t *testing.T) {
	latest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2026.909.1"})
	}))
	defer latest.Close()
	compare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "behind"})
	}))
	defer compare.Close()

	restoreLatest := updater.SetLatestReleaseAPIURL(latest.URL)
	defer restoreLatest()
	restoreCompare := updater.SetCompareReleaseAPIURL(compare.URL)
	defer restoreCompare()

	res := updater.CheckWithCommit(context.Background(), "v2026.716.0-dev-f8bf95c", strings.Repeat("b", 40))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if !res.Newer || res.Notice() == nil || res.LocalCommitRelation != "behind" {
		t.Fatalf("behind development build should update: %+v", res)
	}
}

func TestCheckWithCommit_usesCommitTimeForUnpublishedAheadBuild(t *testing.T) {
	latest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2026.909.1"})
	}))
	defer latest.Close()
	compare := httptest.NewServer(http.NotFoundHandler())
	defer compare.Close()
	releaseCommit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit": map[string]any{
				"committer": map[string]string{"date": "2026-09-09T16:00:00Z"},
			},
		})
	}))
	defer releaseCommit.Close()

	restoreLatest := updater.SetLatestReleaseAPIURL(latest.URL)
	defer restoreLatest()
	restoreCompare := updater.SetCompareReleaseAPIURL(compare.URL)
	defer restoreCompare()
	restoreCommit := updater.SetReleaseCommitAPIURL(releaseCommit.URL)
	defer restoreCommit()

	res := updater.CheckWithCommitTime(context.Background(), "v2026.716.0-dev-local", strings.Repeat("c", 40), time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Newer || res.Notice() != nil || res.LocalCommitRelation != "ahead" {
		t.Fatalf("new unpublished commit should stay untouched: %+v", res)
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

func TestCheck_githubLatestUsesAuthToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
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

func TestInstallFallbackMessage(t *testing.T) {
	t.Parallel()
	msg := updater.InstallFallbackMessage("v2026.602.1")
	if !strings.Contains(msg, "If automatic upgrade fails") {
		t.Fatalf("missing disclaimer: %q", msg)
	}
	if !strings.Contains(msg, "v2026.602.1") {
		t.Fatalf("tag missing from %q", msg)
	}
	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(msg, "irm") {
			t.Fatalf("expected Windows install command in %q", msg)
		}
	case "linux", "darwin":
		if !strings.Contains(msg, "curl") {
			t.Fatalf("expected Unix install command in %q", msg)
		}
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
