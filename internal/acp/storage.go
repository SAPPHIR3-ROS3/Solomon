package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

func (s *Server) discover(id string) error {
	if s.storage == "" || !validID(id) || s.sessions[id] != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(s.storage, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved record
	if err = json.Unmarshal(data, &saved); err != nil {
		return err
	}
	if saved.ID != id {
		return fmt.Errorf("saved session ID mismatch")
	}
	s.sessions[id] = &session{cwd: saved.CWD, record: saved}
	return nil
}
func (s *Server) activate(v *session) error {
	if s.storage == "" || v.lock != nil {
		return nil
	}
	lock := flock.New(filepath.Join(s.storage, v.record.ID+".lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session is active in another ACP process")
	}
	data, err := os.ReadFile(filepath.Join(s.storage, v.record.ID+".json"))
	if err != nil {
		lock.Unlock()
		return err
	}
	var saved record
	if err = json.Unmarshal(data, &saved); err != nil {
		lock.Unlock()
		return err
	}
	if saved.ID != v.record.ID || saved.CWD != v.cwd {
		lock.Unlock()
		return fmt.Errorf("saved session identity mismatch")
	}
	v.record = saved
	v.lock = lock
	return nil
}
func (v *session) release() {
	if v.resourceCancel != nil {
		v.resourceCancel()
	}
	if v.lock != nil {
		_ = v.lock.Unlock()
		v.lock = nil
	}
}

func (s *Server) refresh() error {
	if s.storage == "" {
		return nil
	}
	entries, err := os.ReadDir(s.storage)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 37 || name[32:] != ".json" || !validID(name[:32]) {
			continue
		}
		id := name[:32]
		present[id] = true
		if v := s.sessions[id]; v != nil {
			if v.lock != nil || v.cancel != nil || v.closing {
				continue
			}
			delete(s.sessions, id)
		}
		if err := s.discover(id); err != nil {
			return err
		}
	}
	for id, v := range s.sessions {
		if !present[id] && v.lock == nil && v.cancel == nil && !v.closing {
			delete(s.sessions, id)
		}
	}
	return nil
}
