package staging

import (
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func SessionDir(projectHex, sessionID string) (string, error) {
	d, err := chatstore.ChatsDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, sessionID, "staging"), nil
}
