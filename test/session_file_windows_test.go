package test

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWriteActiveRegistrySnapshotRetriesWindowsReaderLock(t *testing.T) {
	path := sessionSnapshotPath(t)
	if err := writeActiveRegistrySnapshot(path, []byte("running")); err != nil {
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
	err = writeActiveRegistrySnapshot(path, []byte("done"))
	<-released
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if status := snapshotContent(t, data); status != "done" {
		t.Fatalf("status = %q", status)
	}
}
