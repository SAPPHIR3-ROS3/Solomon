package pathglob

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Match(path, pattern string) (bool, error) {
	path = cleanPath(path)
	raw := cleanPath(pattern)
	if raw == "" {
		return false, fmt.Errorf("pathglob: empty pattern")
	}
	if !strings.Contains(raw, "/") {
		if strings.Contains(path, "/") {
			return false, nil
		}
		return filepath.Match(raw, path)
	}
	pattern = cleanPath(NormalizePattern(pattern))
	if pattern == "" {
		return false, fmt.Errorf("pathglob: empty pattern")
	}
	if path == "" && pattern == "**" {
		return true, nil
	}
	pSegs := strings.Split(pattern, "/")
	vSegs := strings.Split(path, "/")
	if path == "" {
		vSegs = nil
	}
	ok, err := matchSegments(pSegs, vSegs, 0, 0)
	return ok, err
}

func matchSegments(pSegs, vSegs []string, pi, vi int) (bool, error) {
	for pi < len(pSegs) {
		p := pSegs[pi]
		if p == "**" {
			if pi == len(pSegs)-1 {
				return true, nil
			}
			for j := vi; j <= len(vSegs); j++ {
				if ok, err := matchSegments(pSegs, vSegs, pi+1, j); err != nil {
					return false, err
				} else if ok {
					return true, nil
				}
			}
			return false, nil
		}
		if vi >= len(vSegs) {
			return false, nil
		}
		ok, err := matchOne(p, vSegs[vi])
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		pi++
		vi++
	}
	return vi == len(vSegs), nil
}

func matchOne(pattern, name string) (bool, error) {
	if pattern == "" {
		return name == "", nil
	}
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") || strings.Contains(pattern, "[") {
		return filepath.Match(pattern, name)
	}
	return pattern == name, nil
}
