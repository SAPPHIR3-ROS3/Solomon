package chatstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriteSessionFileReplacesCompleteSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.json")
	for _, content := range []string{`{"messages":[]}`, `{"messages":["new"]}`} {
		if err := writeSessionFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("snapshot = %q, err = %v", got, err)
		}
	}
	files, _ := filepath.Glob(path + ".*.tmp")
	if len(files) != 0 {
		t.Fatalf("staging files left after success: %v", files)
	}
}

func TestPublishSessionFileDoesNotRetryOtherErrors(t *testing.T) {
	want := errors.New("permanent failure")
	err := publishSessionFile("source", "destination", func(string, string) error { return want }, func(time.Duration) { t.Fatal("unexpected retry") })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestWriteSessionFileConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.json")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := writeSessionFile(path, []byte(strings.Repeat(string(rune('a'+i)), 65536))); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 65536 || strings.Trim(string(data), string(data[0])) != "" {
		t.Fatal("concurrent writes published an incomplete or mixed snapshot")
	}
}

func TestWriteSessionFileRetainsSnapshotOnPublishFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	// A nonempty directory cannot be replaced by a file on any supported OS.
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	err := writeSessionFile(path, []byte("unsaved"))
	if err == nil {
		t.Fatal("expected publication failure")
	}
	files, _ := filepath.Glob(path + ".*.tmp")
	if len(files) != 1 || !strings.Contains(err.Error(), files[0]) {
		t.Fatalf("snapshot missing from error: %v, files %v", err, files)
	}
	data, readErr := os.ReadFile(files[0])
	if readErr != nil || string(data) != "unsaved" {
		t.Fatalf("snapshot = %q, err = %v", data, readErr)
	}
	data, readErr = os.ReadFile(filepath.Join(path, "keep"))
	if readErr != nil || string(data) != "original" {
		t.Fatal("original destination changed")
	}
}
