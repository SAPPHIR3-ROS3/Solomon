package test

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorGitBaseline(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-b", "main")
	original := "  original\n\n"
	write("file.txt", original)
	write("binary.bin", "a\x00b")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Initial fixture")
	write("file.txt", "staged\n")
	git("add", "file.txt")
	write("file.txt", "unsaved on disk\n")
	write("new.txt", "new\n")
	write("large.txt", strings.Repeat("x", (2<<20)+1))
	git("add", "large.txt")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Large fixture", "--", "large.txt")
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/file?revision=HEAD&path="
	for _, tc := range []struct {
		path, content string
		repo          bool
		status        int
	}{
		{"file.txt", original, true, 200},
		{"new.txt", "", true, 200},
		{"binary.bin", "", false, 200},
		{"../outside", "", false, 400},
		{"large.txt", "", false, 500},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response, err := http.Get(endpoint + url.QueryEscape(tc.path))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
			if tc.status != http.StatusOK {
				return
			}
			var payload struct {
				Content string
				IsRepo  bool
			}
			decodeServerTestJSON(t, response, &payload)
			if payload.Content != tc.content || payload.IsRepo != tc.repo {
				t.Fatalf("unexpected baseline: %+v", payload)
			}
		})
	}
	git("add", "file.txt")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Updated fixture")
	response, err := http.Get(endpoint + "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Content string
		IsRepo  bool
	}
	decodeServerTestJSON(t, response, &payload)
	if payload.Content != "unsaved on disk\n" {
		t.Fatalf("baseline did not refresh after commit: %+v", payload)
	}
}

func TestEditorBaselineWithoutRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("plain"), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/file?revision=HEAD&path=file.txt"
	response, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var payload struct {
		Content string
		IsRepo  bool
	}
	decodeServerTestJSON(t, response, &payload)
	if payload.IsRepo || payload.Content != "" {
		t.Fatalf("non-repository baseline: %+v", payload)
	}
}
