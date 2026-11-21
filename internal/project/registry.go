package project

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func openRegistry(path string) (*database.Lease, error) {
	ctx := context.Background()
	db, err := database.BorrowData(ctx, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if err := importRegistry(ctx, db.DB, path); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func importRegistry(ctx context.Context, db *sql.DB, path string) error {
	const source = "projectsId.json"
	var imported bool
	query := "SELECT EXISTS(SELECT 1 FROM legacy_import WHERE source = ?)"
	if err := db.QueryRowContext(ctx, query, source).Scan(&imported); err != nil || imported {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, query, source).Scan(&imported); err != nil || imported {
		return err
	}
	legacy, err := readLegacyMap(path)
	if err != nil {
		return err
	}
	for root, id := range legacy {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO project_registration(root_path, project_id) VALUES (?, ?)", root, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO legacy_import(source) VALUES (?)", source); err != nil {
		return err
	}
	return tx.Commit()
}

func LoadMap(path string) (MapFile, error) {
	db, err := openRegistry(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryRegistry(db.DB)
}

func queryRegistry(db *sql.DB) (MapFile, error) {
	result := make(MapFile)
	rows, err := db.Query("SELECT root_path, project_id FROM project_registration")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var root, id string
		if err := rows.Scan(&root, &id); err != nil {
			return nil, err
		}
		result[root] = id
	}
	return result, rows.Err()
}

func registerProject(path, root, id string) error {
	db, err := openRegistry(path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`
INSERT INTO project_registration(root_path, project_id) VALUES (?, ?)
ON CONFLICT(root_path) DO UPDATE SET project_id = excluded.project_id
WHERE length(project_registration.project_id) != 64`, root, id)
	return err
}

func UnregisterProject(path, id string, removeData bool) error {
	db, err := openRegistry(path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if removeData {
		for _, query := range []string{
			"DELETE FROM chat_session WHERE project_id = ?",
			"DELETE FROM chat_legacy_alias WHERE project_id = ?",
			"DELETE FROM chat_import_failure WHERE project_id = ?",
			"DELETE FROM agent_session WHERE project_id = ?",
			"DELETE FROM terminal_chat WHERE project_id = ?",
			"DELETE FROM research_job WHERE project_id = ?",
		} {
			if _, err := tx.Exec(query, id); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec("DELETE FROM project_registration WHERE project_id = ? COLLATE NOCASE", id); err != nil {
		return err
	}
	return tx.Commit()
}

type RegistryDifference struct {
	RootPath string  `json:"rootPath"`
	LegacyID *string `json:"legacyId,omitempty"`
	StoredID *string `json:"storedId,omitempty"`
}

func VerifyLegacyRegistry(ctx context.Context, path string) ([]RegistryDifference, error) {
	legacyInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !legacyInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("legacy project map is not a regular file")
	}
	databasePath, err := database.Path(filepath.Dir(path), database.Data)
	if err != nil {
		return nil, err
	}
	db, err := database.OpenReadOnly(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	legacy, err := readLegacyMap(path)
	if err != nil {
		return nil, err
	}
	stored, err := queryRegistry(db)
	if err != nil {
		return nil, err
	}
	differences := make([]RegistryDifference, 0)
	for root, id := range legacy {
		actual, exists := stored[root]
		if !exists {
			differences = append(differences, RegistryDifference{RootPath: root, LegacyID: &id})
		} else if actual != id {
			differences = append(differences, RegistryDifference{RootPath: root, LegacyID: &id, StoredID: &actual})
		}
	}
	for root, id := range stored {
		if _, exists := legacy[root]; !exists {
			differences = append(differences, RegistryDifference{RootPath: root, StoredID: &id})
		}
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].RootPath < differences[j].RootPath })
	return differences, nil
}
