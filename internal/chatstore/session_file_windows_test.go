package chatstore

import (
	"os"
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
