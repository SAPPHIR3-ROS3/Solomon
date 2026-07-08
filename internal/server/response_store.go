package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type persistedResponse struct {
	ID           string        `json:"id"`
	Object       string        `json:"object"`
	Conversation string        `json:"conversation,omitempty"`
	Status       string        `json:"status"`
	OutputText   string        `json:"output_text,omitempty"`
	Events       []streamEvent `json:"events"`
	CompletedAt  time.Time     `json:"completed_at"`
}

type responseStore struct {
	mu   sync.RWMutex
	dir  string
	mem  map[string]*persistedResponse
}

func newResponseStore(projHex string) (*responseStore, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "server", "responses", projHex)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &responseStore{dir: dir, mem: map[string]*persistedResponse{}}, nil
}

func (s *responseStore) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *responseStore) Put(rec persistedResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := rec
	cp.Events = append([]streamEvent(nil), rec.Events...)
	s.mem[rec.ID] = &cp
	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(rec.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(rec.ID))
}

func (s *responseStore) Get(id string) (*persistedResponse, bool) {
	s.mu.RLock()
	if rec, ok := s.mem[id]; ok {
		s.mu.RUnlock()
		return rec, true
	}
	s.mu.RUnlock()
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, false
	}
	var rec persistedResponse
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, false
	}
	s.mu.Lock()
	s.mem[id] = &rec
	s.mu.Unlock()
	return &rec, true
}

func (s *responseStore) Snapshot(rec persistedResponse) map[string]any {
	out := map[string]any{
		"id":     rec.ID,
		"object": rec.Object,
		"status": rec.Status,
	}
	if rec.Conversation != "" {
		out["conversation"] = rec.Conversation
	}
	if rec.OutputText != "" {
		out["output_text"] = rec.OutputText
	}
	return out
}
