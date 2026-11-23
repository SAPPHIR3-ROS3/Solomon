package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type FilePosition struct {
	Top          float64  `json:"top"`
	Left         float64  `json:"left"`
	Anchor       int      `json:"anchor"`
	Head         int      `json:"head"`
	ScrollAnchor *int     `json:"scrollAnchor,omitempty"`
	ScrollOffset *float64 `json:"scrollOffset,omitempty"`
	Page         int      `json:"page,omitempty"`
	Layout       string   `json:"layout,omitempty"`
}

func ReadFilePosition(ctx context.Context, home, projectID, path, view string) (*FilePosition, error) {
	db, err := BorrowData(ctx, home)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var payload string
	err = db.QueryRowContext(ctx, "SELECT position FROM project_file_position WHERE project_id = ? COLLATE NOCASE AND path = ? AND view = ?", projectID, path, view).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var position FilePosition
	if err := json.Unmarshal([]byte(payload), &position); err != nil {
		return nil, err
	}
	return &position, nil
}

func WriteFilePosition(ctx context.Context, home, projectID, path, view string, position FilePosition) error {
	payload, err := json.Marshal(position)
	if err != nil {
		return err
	}
	db, err := BorrowData(ctx, home)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `
INSERT INTO project_file_position (project_id, path, view, position)
SELECT project_id, ?, ?, ? FROM project_registration WHERE project_id = ? COLLATE NOCASE LIMIT 1
ON CONFLICT (project_id, path, view) DO UPDATE SET position = excluded.position
`, path, view, string(payload), projectID)
	return err
}
