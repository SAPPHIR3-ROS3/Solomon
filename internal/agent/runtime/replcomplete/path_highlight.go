package replcomplete

import (
	"os"
	"path/filepath"
)

func PathHighlightStatus(projRoot, rawToken string) (exists, isPrefix bool) {
	token := pathTokenForResolve(rawToken)
	searchDir, base, ok := resolvePathForCompletion(projRoot, token)
	if !ok || searchDir == "" {
		return false, false
	}
	if base != "" {
		full := filepath.Join(searchDir, base)
		if st, err := os.Stat(full); err == nil {
			_ = st
			return true, false
		}
	} else if st, err := os.Stat(searchDir); err == nil && st.IsDir() {
		return true, false
	}
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return false, false
	}
	if base == "" {
		return true, false
	}
	for _, e := range entries {
		if matchNamePrefixLen(e.Name(), base) >= 0 {
			return false, true
		}
	}
	return false, false
}
