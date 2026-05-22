package tooloutput

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

var spillNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeSpillToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "none"
	}
	return spillNameSanitizer.ReplaceAllString(s, "_")
}

func writeSpill(projectHex, sessionID, toolCallID, toolName string, data []byte) (string, error) {
	dir, err := chatstore.TempDir(projectHex)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ext := ".txt"
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		ext = ".json"
	}
	name := fmt.Sprintf("%s-%s-%s-%d%s",
		sanitizeSpillToken(sessionID),
		sanitizeSpillToken(toolCallID),
		sanitizeSpillToken(toolName),
		time.Now().UnixNano(),
		ext,
	)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func CleanupProjectTemp(projectHex string) error {
	dir, err := chatstore.TempDir(projectHex)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}
