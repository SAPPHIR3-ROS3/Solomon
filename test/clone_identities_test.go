package test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server/clone"
)

func TestCloneIdentitiesBeforeRepositoryURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	directory := t.TempDir()
	executable := filepath.Join(directory, "gh")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s' '{\"hosts\":{\"github.com\":[{\"login\":\"example-user\"}]}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte("#!/bin/sh\nprintf 'username=aaa-helper\\npassword=fixture\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("HOME", directory)
	if err := os.Mkdir(filepath.Join(directory, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{"id_ed25519": "fixture", "id_ed25519.pub": "ssh-ed25519 fixture aaa@example.com"} {
		if err := os.WriteFile(filepath.Join(directory, ".ssh", name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"", "?url=https%3A%2F%2Fgithub.com%2Fowner%2Frepo"} {
		response := httptest.NewRecorder()
		clone.Handle(response, httptest.NewRequest("GET", "/__solomon/git-clone"+query, nil))
		var identities []clone.Identity
		if err := json.Unmarshal(response.Body.Bytes(), &identities); err != nil {
			t.Fatal(err)
		}
		expectedCount := 3
		if query != "" {
			expectedCount = 3
		}
		if len(identities) != expectedCount || identities[0].ID != "gh:github.com:example-user" || identities[0].Label != "example-user · github.com (GitHub)" {
			t.Fatalf("unexpected identities: %+v", identities)
		}
		if identities[1].ID != "helper" && identities[1].ID != "helper:github.com" {
			t.Fatalf("Git credentials must follow provider identities: %+v", identities)
		}
		if identities[expectedCount-1].ID != "id_ed25519" || identities[expectedCount-1].Label != "aaa@example.com · id_ed25519 (SSH key)" {
			t.Fatalf("SSH identity missing: %+v", identities)
		}
	}
}
