package acp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func (s *Server) SetDatabaseStorage(ctx context.Context, home string) error {
	dir := filepath.Join(home, "acp", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	db, err := database.Open(ctx, home, database.Data)
	if err != nil {
		return err
	}
	s.storage, s.storageDB, s.storageHome = dir, db, home
	if err := s.refresh(); err != nil {
		db.Close()
		return err
	}
	return nil
}

func (s *Server) CloseStorage() {
	if s.storageDB != nil {
		_ = s.storageDB.Close()
	}
}

func (s *Server) saveDatabaseRecord(value record) error {
	events := value.History
	value.History = nil
	metadata, err := json.Marshal(value)
	if err != nil {
		return err
	}
	metadata, err = s.transformArtifacts(metadata, false)
	if err != nil {
		return err
	}
	tx, err := s.storageDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
INSERT INTO acp_session(public_id, cwd, updated_at, metadata) VALUES (?, ?, ?, ?)
ON CONFLICT(public_id) DO UPDATE SET cwd=excluded.cwd, updated_at=excluded.updated_at, metadata=excluded.metadata`,
		value.ID, value.CWD, value.UpdatedAt, string(metadata)); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM acp_history WHERE public_id = ?", value.ID); err != nil {
		return err
	}
	for position, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		data, err = s.transformArtifacts(data, false)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO acp_history(public_id, position, event) VALUES (?, ?, ?)", value.ID, position, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Server) readDatabaseRecord(id string) (record, bool, error) {
	var saved record
	tx, err := s.storageDB.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return saved, false, err
	}
	defer tx.Rollback()
	var metadata sql.NullString
	if err := tx.QueryRow("SELECT metadata FROM acp_session WHERE public_id = ?", id).Scan(&metadata); errors.Is(err, sql.ErrNoRows) {
		return saved, false, nil
	} else if err != nil {
		return saved, false, err
	}
	if !metadata.Valid {
		return saved, true, os.ErrNotExist
	}
	raw, err := s.transformArtifacts([]byte(metadata.String), true)
	if err != nil {
		return saved, true, err
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return saved, true, err
	}
	rows, err := tx.Query("SELECT event FROM acp_history WHERE public_id = ? ORDER BY position", id)
	if err != nil {
		return saved, true, err
	}
	defer rows.Close()
	saved.History = make([]Update, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return saved, true, err
		}
		raw, err := s.transformArtifacts([]byte(data), true)
		if err != nil {
			return saved, true, err
		}
		var event Update
		if err := json.Unmarshal(raw, &event); err != nil {
			return saved, true, err
		}
		saved.History = append(saved.History, event)
	}
	return saved, true, rows.Err()
}

func (s *Server) eraseDatabaseRecord(id string) error {
	tx, err := s.storageDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM acp_history WHERE public_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO acp_session(public_id) VALUES (?)
ON CONFLICT(public_id) DO UPDATE SET cwd=NULL, updated_at=NULL, metadata=NULL`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) transformArtifacts(raw []byte, restore bool) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := s.walkArtifacts(value, restore); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (s *Server) walkArtifacts(value any, restore bool) error {
	switch node := value.(type) {
	case []any:
		for _, item := range node {
			if err := s.walkArtifacts(item, restore); err != nil {
				return err
			}
		}
	case map[string]any:
		kind, _ := node["type"].(string)
		if kind == "image" || kind == "audio" {
			if err := s.transformArtifactField(node, "data", restore); err != nil {
				return err
			}
		}
		if resource, ok := node["resource"].(map[string]any); ok {
			if err := s.transformArtifactField(resource, "blob", restore); err != nil {
				return err
			}
		}
		for _, item := range node {
			if err := s.walkArtifacts(item, restore); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) transformArtifactField(node map[string]any, field string, restore bool) error {
	key := "_solomon_" + field + "_snapshot"
	if restore {
		id, ok := node[key].(string)
		if !ok {
			return nil
		}
		content, err := database.ReadArtifact(s.storageHome, id)
		if err != nil {
			return err
		}
		node[field] = string(content)
		delete(node, key)
		return nil
	}
	content, ok := node[field].(string)
	if !ok || content == "" {
		return nil
	}
	id, err := database.StoreArtifact(s.storageHome, []byte(content))
	if err != nil {
		return err
	}
	node[key] = id
	delete(node, field)
	return nil
}
