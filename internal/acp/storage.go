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
	saved, err := s.readStoredRecord(id)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
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
	saved, err := s.readStoredRecord(v.record.ID)
	if err != nil {
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
	ids, err := s.storedRecordIDs()
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, id := range ids {
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

func (s *Server) readStoredRecord(id string) (record, error) {
	if s.storageDB != nil {
		saved, known, err := s.readDatabaseRecord(id)
		if known || err != nil {
			return saved, err
		}
	}
	var saved record
	data, err := os.ReadFile(filepath.Join(s.storage, id+".json"))
	if err != nil {
		return saved, err
	}
	err = json.Unmarshal(data, &saved)
	return saved, err
}

func (s *Server) storedRecordIDs() ([]string, error) {
	entries, err := os.ReadDir(s.storage)
	if err != nil {
		return nil, err
	}
	var ids []string
	known := make(map[string]bool)
	if s.storageDB != nil {
		rows, err := s.storageDB.Query("SELECT public_id, metadata IS NOT NULL FROM acp_session")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var active bool
			if err := rows.Scan(&id, &active); err != nil {
				return nil, err
			}
			known[id] = true
			if active {
				ids = append(ids, id)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) == 37 && name[32:] == ".json" && validID(name[:32]) && !known[name[:32]] {
			ids = append(ids, name[:32])
		}
	}
	return ids, nil
}

func (s *Server) eraseStoredRecord(id string) error {
	if s.storageDB != nil {
		return s.eraseDatabaseRecord(id)
	}
	return os.Remove(filepath.Join(s.storage, id+".json"))
}
