package atmention

import (
	"path/filepath"
	"strings"
)

var textExtensions = map[string]struct{}{
	".md": {}, ".txt": {}, ".go": {}, ".json": {}, ".yaml": {}, ".yml": {}, ".toml": {},
	".xml": {}, ".html": {}, ".htm": {}, ".css": {}, ".scss": {}, ".sass": {}, ".less": {},
	".js": {}, ".jsx": {}, ".ts": {}, ".tsx": {}, ".mjs": {}, ".cjs": {},
	".py": {}, ".rb": {}, ".rs": {}, ".java": {}, ".kt": {}, ".kts": {}, ".swift": {},
	".c": {}, ".cc": {}, ".cpp": {}, ".cxx": {}, ".h": {}, ".hh": {}, ".hpp": {}, ".hxx": {},
	".cs": {}, ".fs": {}, ".vb": {}, ".sql": {}, ".graphql": {}, ".gql": {}, ".proto": {},
	".sh": {}, ".bash": {}, ".zsh": {}, ".fish": {}, ".ps1": {}, ".bat": {}, ".cmd": {},
	".tmpl": {}, ".tpl": {}, ".mustache": {}, ".ini": {}, ".cfg": {}, ".conf": {}, ".env": {},
	".properties": {}, ".gradle": {}, ".cmake": {}, ".mk": {}, ".dockerfile": {},
	".vue": {}, ".svelte": {}, ".astro": {}, ".lua": {}, ".php": {}, ".pl": {}, ".pm": {},
	".r": {}, ".R": {}, ".ex": {}, ".exs": {}, ".erl": {}, ".hrl": {}, ".clj": {}, ".cljs": {},
	".hs": {}, ".lhs": {}, ".ml": {}, ".mli": {}, ".zig": {}, ".v": {}, ".sv": {},
	".mod": {}, ".sum": {}, ".lock": {}, ".editorconfig": {}, ".gitignore": {}, ".gitattributes": {},
}

var textBasenames = map[string]struct{}{
	"makefile": {}, "dockerfile": {}, "license": {}, "readme": {}, "changelog": {},
	"cmakelists.txt": {}, "procfile": {}, "brewfile": {}, "gemfile": {}, "rakefile": {},
	"vagrantfile": {}, "justfile": {}, "taskfile": {},
}

func isAllowedTextFile(name string, data []byte) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != "" {
		if _, ok := textExtensions[ext]; ok {
			return !isBinary(data)
		}
		return false
	}
	base := strings.ToLower(strings.TrimSpace(name))
	if _, ok := textBasenames[base]; ok {
		return !isBinary(data)
	}
	return false
}
