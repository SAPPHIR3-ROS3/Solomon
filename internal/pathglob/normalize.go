package pathglob

import "strings"

func NormalizePattern(pattern string) string {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return "**/*"
	}
	if strings.HasPrefix(p, "/") {
		p = strings.TrimPrefix(p, "/")
	}
	if strings.HasPrefix(p, "**/") || strings.HasPrefix(p, "**\\") {
		return strings.ReplaceAll(p, "\\", "/")
	}
	return "**/" + strings.ReplaceAll(p, "\\", "/")
}

func cleanPath(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	for strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
	}
	return strings.Trim(p, "/")
}
