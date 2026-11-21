package chatstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/ncruces/go-sqlite3"
)

func RunChatMigration(ctx context.Context) {
	if !migrationPause(ctx, 3*time.Second) {
		return
	}
	root, err := paths.ProjectsDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	db, home, err := openChatDatabase()
	if err != nil {
		return
	}
	defer db.Close()
	exists, err := db.PrepareContext(ctx, `
SELECT EXISTS(SELECT 1 FROM chat_legacy_alias WHERE project_id = ? AND chat_id = ?)
OR EXISTS(SELECT 1 FROM chat_import_failure
          WHERE project_id = ? AND chat_id = ? AND file_mtime_ns = ? AND file_size = ?)`)
	if err != nil {
		return
	}
	defer exists.Close()
	failure, err := db.PrepareContext(ctx, `
INSERT INTO chat_import_failure(project_id, chat_id, file_mtime_ns, file_size, description)
VALUES (?, ?, ?, ?, ?) ON CONFLICT(project_id, chat_id) DO UPDATE SET
file_mtime_ns = excluded.file_mtime_ns, file_size = excluded.file_size, description = excluded.description`)
	if err != nil {
		return
	}
	defer failure.Close()
	processed := 0
	stopAt := time.Now().Add(40 * time.Millisecond)
	for _, folder := range entries {
		if !folder.IsDir() {
			continue
		}
		project := folder.Name()
		path, err := ChatsDir(project)
		if err != nil {
			continue
		}
		files, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, file := range files {
			if ctx.Err() != nil {
				return
			}
			if file.Type().IsRegular() && strings.HasSuffix(file.Name(), ".json") {
				id := strings.TrimSuffix(file.Name(), ".json")
				for {
					for interactiveChatWrites.Load() > 0 {
						if !migrationPause(ctx, 150*time.Millisecond) {
							return
						}
					}
					processedFile, retry := migrateCandidate(ctx, db.DB, exists, failure, home, project, id, filepath.Join(path, file.Name()), file)
					if retry {
						if !migrationPause(ctx, 150*time.Millisecond) {
							return
						}
						continue
					}
					if processedFile {
						processed++
					}
					break
				}
				if processed >= 4 || time.Now().After(stopAt) {
					if !migrationPause(ctx, 250*time.Millisecond) {
						return
					}
					processed = 0
					stopAt = time.Now().Add(40 * time.Millisecond)
				}
			}
		}
	}
}

func migrationPause(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func migrateCandidate(ctx context.Context, db *sql.DB, exists, failure *sql.Stmt, home, project, id, path string, file fs.DirEntry) (processed, retry bool) {
	info, err := file.Info()
	if err != nil {
		return false, false
	}
	var skip bool
	err = exists.QueryRowContext(ctx, project, id, project, id, info.ModTime().UnixNano(), info.Size()).Scan(&skip)
	if err != nil || skip {
		return false, false
	}
	session, err := readLegacyChat(path)
	if err == nil && session.ID != id {
		err = fs.ErrInvalid
	}
	if err == nil {
		FinishSessionLoad(session)
		err = storeBackgroundChat(ctx, db, home, project, session)
		if errors.Is(err, sqlite3.BUSY) || errors.Is(err, sqlite3.LOCKED) || errors.Is(err, errImportYielded) {
			return false, true
		}
	}
	if err != nil && ctx.Err() == nil {
		_, recordErr := failure.ExecContext(ctx, project, id, info.ModTime().UnixNano(), info.Size(), err.Error())
		if recordErr != nil {
			logging.Diagnostic("chat import failure could not be recorded", map[string]any{"error": recordErr.Error()})
		}
	}
	return true, false
}

var errImportYielded = errors.New("chat import yielded to interactive save")

func storeBackgroundChat(ctx context.Context, db *sql.DB, home, project string, session *Session) error {
	if interactiveChatWrites.Load() != 0 {
		return errImportYielded
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 25"); err != nil {
		return err
	}
	defer func() {
		reset, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err := conn.ExecContext(reset, "PRAGMA busy_timeout = 5000"); err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	importCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-importCtx.Done():
				return
			case <-ticker.C:
				if interactiveChatWrites.Load() != 0 {
					cancel()
					return
				}
			}
		}
	}()
	err = storeChatWithDB(importCtx, conn, home, project, session, true)
	if err != nil && importCtx.Err() != nil && ctx.Err() == nil {
		return errImportYielded
	}
	return err
}
