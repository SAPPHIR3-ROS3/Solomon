package instructions

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skipDirNames = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {}, ".solomon": {},
}

func NormalizeRelDir(projRoot, absPath string) string {
	projRoot = filepath.Clean(projRoot)
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(projRoot, absPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

func ActivateDirsFromAbsPath(projRoot, absPath string) []string {
	projRoot = filepath.Clean(projRoot)
	absPath = filepath.Clean(absPath)
	dir := absPath
	if st, err := os.Stat(absPath); err == nil && !st.IsDir() {
		dir = filepath.Dir(absPath)
	}
	var found []string
	for {
		rel := NormalizeRelDir(projRoot, dir)
		if rel == "" {
			break
		}
		if rel != "." && rel != "" {
			if _, skip := skipDirNames[filepath.Base(dir)]; !skip {
				if _, ok := FindAgentsFile(dir); ok {
					found = append(found, rel)
				}
			}
		}
		if dir == projRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return found
}

func MergeActivatedDirs(existing, newly []string) []string {
	set := make(map[string]struct{}, len(existing)+len(newly))
	for _, d := range existing {
		d = strings.TrimSpace(filepath.ToSlash(d))
		if d != "" && d != "." {
			set[d] = struct{}{}
		}
	}
	for _, d := range newly {
		d = strings.TrimSpace(filepath.ToSlash(d))
		if d != "" && d != "." {
			set[d] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
