package instructions

import (
	"path/filepath"
	"regexp"
	"strings"
)

var shellPathTokenRe = regexp.MustCompile(`(?:\./|\../|[A-Za-z]:\\|[A-Za-z0-9_.-]+/)[A-Za-z0-9_./\\-]+`)

func PathsFromShellCommand(projRoot, command string) []string {
	command = strings.TrimSpace(command)
	if command == "" || projRoot == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, tok := range shellPathTokenRe.FindAllString(command, -1) {
		tok = strings.Trim(tok, `"'`)
		tok = strings.TrimRight(tok, ",;|&")
		if tok == "" || tok == "." || tok == "./" {
			continue
		}
		abs := tok
		if !filepath.IsAbs(tok) {
			abs = filepath.Join(projRoot, filepath.FromSlash(tok))
		}
		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		for _, rel := range ActivateDirsFromAbsPath(projRoot, abs) {
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			out = append(out, rel)
		}
	}
	return out
}
