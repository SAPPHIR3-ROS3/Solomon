package live

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

type Watcher struct {
	Path        string
	Current     Config
	seen        [32]byte
	initialized bool
	missing     bool
}

func NewWatcher(path string) *Watcher { return &Watcher{Path: path, Current: Default()} }
func (w *Watcher) Poll() (bool, error) {
	file, err := os.Open(w.Path)
	if os.IsNotExist(err) {
		if w.initialized && w.missing {
			return false, nil
		}
		w.initialized = true
		w.missing = true
		w.Current = Default()
		return true, nil
	}
	if err != nil {
		return false, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, MaxConfigBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return false, readErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	sum := sha256.Sum256(data)
	if w.initialized && !w.missing && sum == w.seen {
		return false, nil
	}
	w.initialized = true
	w.missing = false
	w.seen = sum
	candidate, err := Decode(data)
	if err != nil {
		return false, fmt.Errorf("keeping last valid TUI configuration: %w", err)
	}
	w.Current = candidate
	return true, nil
}
