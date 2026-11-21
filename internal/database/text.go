package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func CaptureText(ctx context.Context, db *sql.DB, path string) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	content, err := os.ReadFile(absolute)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO managed_text(path, content, content_sha256) VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET content = excluded.content, content_sha256 = excluded.content_sha256
WHERE managed_text.content_sha256 != excluded.content_sha256`, absolute, content, digest(content))
	if err != nil {
		return nil, err
	}
	return content, tx.Commit()
}

func RestoreText(ctx context.Context, db *sql.DB, path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var content []byte
	var sha string
	if err := db.QueryRowContext(ctx, "SELECT content, content_sha256 FROM managed_text WHERE path = ?", absolute).Scan(&content, &sha); err != nil {
		return err
	}
	if digest(content) != sha {
		return fmt.Errorf("stored document checksum mismatch")
	}
	file, err := os.CreateTemp(filepath.Dir(absolute), ".solomon-restore-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Link(file.Name(), absolute)
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func HasText(ctx context.Context, db *sql.DB, path string) (bool, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	var exists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM managed_text WHERE path = ?)", absolute).Scan(&exists)
	return exists, err
}
