package paths

import (
	"context"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func EnsureGlobalAgentsPath() (string, error) {
	path, err := GlobalAgentsPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	home := filepath.Dir(path)
	dbPath, err := database.Path(home, database.Data)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dbPath); err == nil {
		ctx := context.Background()
		db, err := database.BorrowData(ctx, home)
		if err != nil {
			return "", err
		}
		preserved, err := database.HasText(ctx, db.DB, path)
		db.Close()
		if err != nil {
			return "", err
		}
		if preserved {
			return path, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	return path, file.Close()
}

func ReadGlobalAgents(ctx context.Context, path string) ([]byte, error) {
	db, err := database.BorrowData(ctx, filepath.Dir(path))
	if err == nil {
		defer db.Close()
		if content, err := database.CaptureText(ctx, db.DB, path); err == nil {
			return content, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func RestoreGlobalAgents(ctx context.Context) error {
	path, err := GlobalAgentsPath()
	if err != nil {
		return err
	}
	db, err := database.BorrowData(ctx, filepath.Dir(path))
	if err != nil {
		return err
	}
	defer db.Close()
	return database.RestoreText(ctx, db.DB, path)
}
