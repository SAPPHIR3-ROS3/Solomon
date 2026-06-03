package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
