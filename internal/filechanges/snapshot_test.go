package filechanges

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFileChangesCompareTurnOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("unchanged.txt", "preexisting changes\n")
	write("nested/edit.txt", "old\nretained\n")
	write("delete.txt", "deleted\n")
	write("binary.bin", "\x00a")
	before, err := Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	write("nested/edit.txt", "new\nretained\nextra\n")
	write("new.txt", "created\n")
	write("empty.txt", "")
	write("binary.bin", "\x00b")
	if err := os.Remove(filepath.Join(root, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	after, err := Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	changes, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Fatalf("changes: %+v", changes)
	}
	got := map[string]Change{}
	for _, c := range changes {
		got[c.Name] = c
	}
	if c := got["nested/edit.txt"]; c.Added != 2 || c.Removed != 1 {
		t.Fatalf("edit: %+v", c)
	}
	if got["delete.txt"].Removed != 1 || got["new.txt"].Added != 1 || !got["binary.bin"].Binary {
		t.Fatalf("changes: %+v", changes)
	}
	same, err := Compare(after, after)
	if err != nil || len(same) != 0 {
		t.Fatalf("unchanged: %+v %v", same, err)
	}
}
func TestFileChangesRespectGitIgnore(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Skip(err)
	}
	for name, body := range map[string]string{".gitignore": "ignored.txt\n", "ignored.txt": "secret", "visible.txt": "visible"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if snapshot.files["ignored.txt"] || !snapshot.files["visible.txt"] {
		t.Fatalf("files: %v", snapshot.files)
	}
}

func TestFileChangesSelectedPathsSkipInaccessibleSiblings(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	blocked := filepath.Join(root, "adguard", "workdir", "data")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0700)
	target := filepath.Join(desktop, "test.txt")
	before, err := CapturePaths(root, []string{target})
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	if err := os.WriteFile(target, []byte("own line\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := CapturePaths(root, []string{target})
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	changes, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Name != "Desktop/test.txt" || changes[0].Added != 1 {
		t.Fatalf("desktop file missing: %+v", changes)
	}
	// The fallback project walk must also survive an inaccessible sibling.
	all, err := Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	defer all.Close()
	if !all.files[filepath.Join("Desktop", "test.txt")] {
		t.Fatal("permission failure discarded readable files")
	}
}
func TestFileChangesExplicitExternalAndIgnoredFile(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ignored, external := filepath.Join(root, "ignored.txt"), filepath.Join(outside, "external.txt")
	before, err := CapturePaths(root, []string{ignored, external})
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	for _, path := range []string{ignored, external} {
		if err := os.WriteFile(path, []byte("own\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	after, err := CapturePaths(root, []string{ignored, external})
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	changes, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("explicit targets: %+v", changes)
	}
}
