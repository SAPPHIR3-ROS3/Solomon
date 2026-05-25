package cursor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

const (
	DefaultPort     = 8766
	IntegrationName = "cursor"
)

func InstallDir() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SOLOMON_CURSOR_API_ROOT")); p != "" {
		return p, nil
	}
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "integrations", IntegrationName), nil
}

func EntryScript(dir string) string {
	return filepath.Join(dir, "dist", "index.js")
}

func NodeModulesDir(dir string) string {
	return filepath.Join(dir, "node_modules")
}

func DefaultBaseURL(port int) string {
	if port <= 0 {
		port = DefaultPort
	}
	return "http://127.0.0.1:" + itoa(port) + "/v1/"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
