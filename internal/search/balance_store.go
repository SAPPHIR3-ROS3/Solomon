package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// BalanceState is the persisted scheduler state for the MCP-backed search
// adapters. Counts represent backend attempts, including attempts that later
// fail and cause a fallback.
type BalanceState struct {
	Month  string           `json:"month"`
	Counts map[string]int64 `json:"counts"`
	Next   string           `json:"next,omitempty"`
}

// BalanceStore provides an atomic read-modify-write operation. The file
// implementation uses a process lock as well as an OS file lock so two
// Solomon processes do not lose increments made in the same month.
type BalanceStore interface {
	Update(func(*BalanceState) error) (BalanceState, error)
}

type MemoryBalanceStore struct {
	mu    sync.Mutex
	state BalanceState
}

func NewMemoryBalanceStore(initial BalanceState) *MemoryBalanceStore {
	return &MemoryBalanceStore{state: cloneBalanceState(initial)}
}

func (s *MemoryBalanceStore) Update(fn func(*BalanceState) error) (BalanceState, error) {
	if s == nil {
		return BalanceState{}, errors.New("nil memory balance store")
	}
	if fn == nil {
		return BalanceState{}, errors.New("balance store update function is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneBalanceState(s.state)
	if err := fn(&next); err != nil {
		return BalanceState{}, err
	}
	s.state = cloneBalanceState(next)
	return cloneBalanceState(next), nil
}

func (s *MemoryBalanceStore) Snapshot() BalanceState {
	if s == nil {
		return BalanceState{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneBalanceState(s.state)
}

type FileBalanceStore struct {
	path string
	mu   sync.Mutex
}

func NewFileBalanceStore(path string) (*FileBalanceStore, error) {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil, errors.New("balance store path is required")
	}
	return &FileBalanceStore{path: path}, nil
}

func DefaultBalanceStore() BalanceStore {
	path, err := paths.WebSearchBalancePath()
	if err != nil {
		return NewMemoryBalanceStore(BalanceState{})
	}
	store, err := NewFileBalanceStore(path)
	if err != nil {
		return NewMemoryBalanceStore(BalanceState{})
	}
	return store
}

func (s *FileBalanceStore) Update(fn func(*BalanceState) error) (BalanceState, error) {
	if s == nil || s.path == "" {
		return BalanceState{}, errors.New("nil file balance store")
	}
	if fn == nil {
		return BalanceState{}, errors.New("balance store update function is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return BalanceState{}, fmt.Errorf("create balance store directory: %w", err)
	}

	lock := flock.New(s.path + ".lock")
	if err := lock.Lock(); err != nil {
		return BalanceState{}, fmt.Errorf("lock balance store: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	state, err := readBalanceState(s.path)
	if err != nil {
		return BalanceState{}, err
	}
	if err := fn(&state); err != nil {
		return BalanceState{}, err
	}
	if err := writeBalanceState(s.path, state); err != nil {
		return BalanceState{}, err
	}
	return cloneBalanceState(state), nil
}

func readBalanceState(path string) (BalanceState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return BalanceState{Counts: map[string]int64{}}, nil
		}
		return BalanceState{}, fmt.Errorf("read balance store: %w", err)
	}
	if len(b) == 0 {
		return BalanceState{Counts: map[string]int64{}}, nil
	}
	var state BalanceState
	if err := json.Unmarshal(b, &state); err != nil {
		return BalanceState{}, fmt.Errorf("decode balance store: %w", err)
	}
	if state.Counts == nil {
		state.Counts = map[string]int64{}
	}
	return state, nil
}

func writeBalanceState(path string, state BalanceState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode balance store: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".websearch-usage-*")
	if err != nil {
		return fmt.Errorf("create balance store temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("set balance store permissions: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		return fmt.Errorf("write balance store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close balance store temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace balance store: %w", err)
	}
	return nil
}

func cloneBalanceState(state BalanceState) BalanceState {
	copy := state
	copy.Counts = make(map[string]int64, len(state.Counts))
	for key, value := range state.Counts {
		copy.Counts[key] = value
	}
	return copy
}
