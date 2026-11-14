package chatstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Each writer owns its staging file. A failed publication keeps the complete
// snapshot for diagnosis/recovery instead of silently discarding unsaved data.
func writeSessionFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(f.Name())
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := publishSessionFile(f.Name(), path, os.Rename, time.Sleep); err != nil {
		return fmt.Errorf("save chat failed; unsaved snapshot retained at %s: %w", f.Name(), err)
	}
	return nil
}

func publishSessionFile(source, destination string, rename func(string, string) error, sleep func(time.Duration)) error {
	deadline := time.Now().Add(2 * time.Second)
	for delay := 10 * time.Millisecond; ; delay = min(delay*2, 200*time.Millisecond) {
		err := rename(source, destination)
		if err == nil || !retrySessionRename(err) || time.Now().Add(delay).After(deadline) {
			return err
		}
		sleep(delay)
	}
}
