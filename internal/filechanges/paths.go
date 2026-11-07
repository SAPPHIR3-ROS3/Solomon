package filechanges

import (
	"path/filepath"
	"strings"
)

func IsInternalSnapshotPath(path string) bool {
	if snapshotPath(path) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && snapshotPath(resolved)
}

func snapshotPath(path string) bool {
	for _, part := range strings.FieldsFunc(filepath.Clean(path), func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.HasPrefix(part, "solomon-changes-") {
			return true
		}
	}
	return false
}
