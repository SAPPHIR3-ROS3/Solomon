package chatstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var terminalIDPattern = regexp.MustCompile(`^terminal-[a-z0-9-]{1,64}$`)

func ValidTerminalID(id string) bool { return terminalIDPattern.MatchString(id) }

func terminalChatPath(id string) (string, error) {
	if !ValidTerminalID(id) {
		return "", fmt.Errorf("invalid terminal session ID")
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "run", "terminal-chats", id+".json"), nil
}

type terminalChat struct {
	Project string `json:"project"`
	Chat    string `json:"chat"`
}

// RememberTerminalChat keeps each terminal's active chat separate, including
// when several terminals share the same project during an upgrade.
func RememberTerminalChat(id, project, chat string) error {
	if id == "" {
		return nil
	}
	if !ValidTerminalID(id) {
		return fmt.Errorf("invalid terminal session ID")
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`
INSERT INTO terminal_chat(terminal_id, project_id, chat_id) VALUES (?, ?, ?)
ON CONFLICT(terminal_id) DO UPDATE SET project_id = excluded.project_id, chat_id = excluded.chat_id`, id, project, chat)
	return err
}

func ReadTerminalChat(id, project string) (*Session, error) {
	if id == "" {
		return nil, os.ErrNotExist
	}
	path, err := terminalChatPath(id)
	if err != nil {
		return nil, err
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var record terminalChat
	err = db.QueryRow("SELECT project_id, chat_id FROM terminal_chat WHERE terminal_id = ?", id).Scan(&record.Project, &record.Chat)
	if errors.Is(err, sql.ErrNoRows) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		if _, err := db.Exec("INSERT OR IGNORE INTO terminal_chat(terminal_id, project_id, chat_id) VALUES (?, ?, ?)", id, record.Project, record.Chat); err != nil {
			return nil, err
		}
		if err := db.QueryRow("SELECT project_id, chat_id FROM terminal_chat WHERE terminal_id = ?", id).Scan(&record.Project, &record.Chat); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if record.Project != project || record.Chat == "" {
		return nil, os.ErrNotExist
	}
	return ReadSession(project, record.Chat)
}
