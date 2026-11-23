package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server/clone"
)

func TestCloneReportsMissingExecutables(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	destination := filepath.Join(t.TempDir(), "clone")
	check := func(identity, message string) {
		t.Helper()
		payload, err := json.Marshal(map[string]string{
			"url": "https://github.com/example/repo.git", "destination": destination, "identity": identity,
		})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		clone.Handle(response, httptest.NewRequest(http.MethodPost, "/__solomon/git-clone", bytes.NewReader(payload)))
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), message) {
			t.Fatalf("got %d %s, want 503 mentioning %q", response.Code, response.Body.String(), message)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatalf("missing executable should leave destination untouched: %v", err)
		}
	}
	check("anonymous", "Git executable not found")
	if runtime.GOOS == "windows" {
		bin += string(os.PathSeparator)
	}
	git := filepath.Join(bin, "git")
	if runtime.GOOS == "windows" {
		git += ".exe"
	}
	if err := os.WriteFile(git, []byte("unused fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	check("gh:github.com:example", "GitHub CLI (gh) not found")
}
