package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type FolderState struct {
	Expanded      []string `json:"expanded"`
	RootCollapsed bool     `json:"rootCollapsed"`
}

func ReadFolderState(ctx context.Context, home, projectID, panel string) (*FolderState, error) {
	db, err := BorrowData(ctx, home)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var payload string
	err = db.QueryRowContext(ctx, "SELECT state FROM project_folder_state WHERE project_id = ? COLLATE NOCASE AND panel = ?", projectID, panel).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state FolderState
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func WriteFolderState(ctx context.Context, home, projectID, panel string, state FolderState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	db, err := BorrowData(ctx, home)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `
INSERT INTO project_folder_state (project_id, panel, state)
SELECT project_id, ?, ? FROM project_registration WHERE project_id = ? COLLATE NOCASE LIMIT 1
ON CONFLICT (project_id, panel) DO UPDATE SET state = excluded.state
`, panel, string(payload), projectID)
	return err
}
