package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var artifactColumns = [][2]string{
	{"chat_session", "metadata"}, {"chat_message", "metadata"},
	{"agent_session", "metadata"}, {"agent_message", "message"},
	{"agent_queue", "operation"}, {"acp_session", "metadata"}, {"acp_history", "event"},
}

func CollectArtifacts(ctx context.Context, home string) (int, error) {
	dir := filepath.Join(home, "media", "chat-snapshots")
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	release, err := AcquireMaintenance(ctx, home)
	if err != nil {
		return 0, err
	}
	defer release()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	current, err := Path(home, Data)
	if err != nil {
		return 0, err
	}
	pins := make(map[string]bool)
	if err := artifactReferences(ctx, current, pins); err != nil {
		return 0, err
	}
	homes, err := os.ReadDir(home)
	if err != nil {
		return 0, err
	}
	for _, entry := range homes {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".update-storage-") {
			continue
		}
		backup := filepath.Join(home, entry.Name(), filepath.Base(current))
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return 0, err
		}
		if err := artifactReferences(ctx, backup, pins); err != nil {
			return 0, err
		}
	}
	removed := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		name := entry.Name()
		if !entry.Type().IsRegular() || !isArtifactID(name) || pins[name] {
			continue
		}
		path := filepath.Join(dir, name)
		match, err := hasArtifactDigest(path, name)
		if err != nil {
			return removed, err
		}
		if match && ctx.Err() == nil {
			if err := os.Remove(path); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, ctx.Err()
}

func artifactReferences(ctx context.Context, path string, pins map[string]bool) error {
	db, err := OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()
	var id, version int
	if err := db.QueryRowContext(ctx, "SELECT * FROM pragma_application_id(), pragma_user_version()").Scan(&id, &version); err != nil {
		return err
	}
	if id != 1397705796 || version < 1 || version > 8 {
		return fmt.Errorf("unsupported artifact reference database: %s", path)
	}
	for _, pair := range artifactColumns {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name=?)", pair[0]).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`
SELECT DISTINCT atom FROM %s AS source, json_tree(source.%s)
WHERE json_tree.type = 'text' AND length(atom) = 64`, pair[0], pair[1]))
		if err != nil {
			return err
		}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return err
			}
			if isArtifactID(value) {
				pins[value] = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func isArtifactID(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func hasArtifactDigest(path, id string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)) == id, err
}

func RunArtifactCollection(ctx context.Context, home string) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			window, cancel := context.WithTimeout(ctx, 2*time.Second)
			_, _ = CollectArtifacts(window, home)
			cancel()
			timer.Reset(15 * time.Minute)
		}
	}
}
