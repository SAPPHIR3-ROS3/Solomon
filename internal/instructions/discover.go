package instructions

import (
	"os"
	"path/filepath"
)

var agentsFileNames = []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"}

func FindAgentsFile(dir string) (filePath string, ok bool) {
	dir = filepath.Clean(dir)
	for _, name := range agentsFileNames {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		return p, true
	}
	return "", false
}
