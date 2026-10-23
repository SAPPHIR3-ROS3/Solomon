package chatstore

import (
	"encoding/json"
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
	path, err := terminalChatPath(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(terminalChat{Project: project, Chat: chat})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".terminal-chat-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func ReadTerminalChat(id, project string) (*Session, error) {
	if id == "" {
		return nil, os.ErrNotExist
	}
	path, err := terminalChatPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record terminalChat
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	if record.Project != project || record.Chat == "" {
		return nil, os.ErrNotExist
	}
	return ReadSession(project, record.Chat)
}
