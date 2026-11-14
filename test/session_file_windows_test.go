package test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWriteSubagentSnapshotRetriesWindowsReaderLock(t *testing.T) {
	path := sessionSnapshotPath(t)
	if err := writeSubagentSnapshot(path, []byte("running")); err != nil {
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
	err = writeSubagentSnapshot(path, []byte("done"))
	<-released
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "done" {
		t.Fatalf("status = %q", snapshot.Status)
	}
}
