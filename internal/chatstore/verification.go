package chatstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type LegacyChatDifference struct {
	Project string `json:"projectId"`
	Chat    string `json:"chatId"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func VerifyLegacyChats(ctx context.Context) ([]LegacyChatDifference, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	path, err := database.Path(home, database.Data)
	if err != nil {
		return nil, err
	}
	db, err := database.OpenReadOnly(ctx, path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	root, err := paths.ProjectsDir()
	if err != nil {
		return nil, err
	}
	result := make([]LegacyChatDifference, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root && errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if entry.IsDir() {
			if len(parts) > 2 || (len(parts) == 2 && parts[1] != "chats") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(parts) != 3 || parts[1] != "chats" || !strings.HasSuffix(parts[2], ".json") {
			return nil
		}
		item := LegacyChatDifference{Project: parts[0], Chat: strings.TrimSuffix(parts[2], ".json")}
		legacy, err := readLegacyChat(path)
		if err != nil {
			item.Status, item.Detail = "invalid-source", err.Error()
		} else if stored, known, err := readStoredChat(db, home, item.Project, item.Chat); err != nil {
			item.Status, item.Detail = "invalid-database-record", err.Error()
		} else if !known {
			item.Status = "pending"
		} else if stored == nil {
			item.Status = "deleted"
		} else {
			FinishSessionLoad(legacy)
			FinishSessionLoad(stored)
			legacy.Capabilities.Backfill(legacy.Messages)
			stored.Capabilities.Backfill(stored.Messages)
			left, leftErr := json.Marshal(legacy)
			right, rightErr := json.Marshal(stored)
			if err := errors.Join(leftErr, rightErr); err != nil {
				item.Status, item.Detail = "comparison-error", err.Error()
			} else if !bytes.Equal(left, right) {
				item.Status = "different"
			}
		}
		if item.Status != "" {
			result = append(result, item)
		}
		return nil
	})
	return result, err
}
