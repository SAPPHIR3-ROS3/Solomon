package chatstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestPublishSessionFileRetriesWindowsFileLocks(t *testing.T) {
	for _, code := range []error{windows.ERROR_ACCESS_DENIED, windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION} {
		calls := 0
		err := publishSessionFile("source", "destination", func(string, string) error {
			calls++
			if calls < 3 {
				return &os.LinkError{Op: "rename", Err: code}
			}
			return nil
		}, func(time.Duration) {})
		if err != nil || calls != 3 {
			t.Fatalf("%v: calls=%d err=%v", code, calls, err)
		}
	}
}

func TestWriteSubagentSnapshotRetriesWindowsReaderLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subchat.json")
	if err := writeJSONAtomic(path, map[string]string{"status": "running"}); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() { time.Sleep(100 * time.Millisecond); windows.CloseHandle(handle); close(released) }()
	err = writeJSONAtomic(path, map[string]string{"status": "done"})
	<-released
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]string
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["status"] != "done" {
		t.Fatalf("status = %q", snapshot["status"])
	}
}
