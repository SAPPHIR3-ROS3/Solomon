package replcomplete

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var pathEnvVarRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

type pathCompletionSpan struct {
	replaceStart int
	pathPrefix   string
	inQuotes     bool
}

func pathCompletionSpanInShell(shell []rune, cursor int) (pathCompletionSpan, bool) {
	if cursor < 0 || cursor > len(shell) {
		return pathCompletionSpan{}, false
	}
	tokStart, _ := shellTokenBounds(shell, cursor)
	raw := string(shell[tokStart:cursor])
	inQuotes, _ := quoteStateAt(shell, cursor)
	if len(raw) > 0 && (raw[0] == '"' || raw[0] == '\'') {
		inQuotes = true
	}
	if raw == "" {
		return pathCompletionSpan{replaceStart: tokStart, inQuotes: inQuotes}, true
	}
	pathVal := pathTokenForResolve(raw)
	replaceStart := tokStart + lastPathComponentOffset(raw)
	return pathCompletionSpan{replaceStart: replaceStart, pathPrefix: pathVal, inQuotes: inQuotes}, true
}

func lastPathComponentOffset(raw string) int {
	if raw == "" {
		return 0
	}
	inner := raw
	off := 0
	if raw[0] == '"' || raw[0] == '\'' {
		q := raw[0]
		inner = raw[1:]
		off = 1
		end := len(inner)
		if end > 0 && inner[end-1] == q {
			end--
		}
		inner = inner[:end]
	}
	last := 0
	for i := 0; i < len(inner); i++ {
		if inner[i] == '/' || inner[i] == '\\' {
			last = i + 1
		}
	}
	return off + last
}

func pathTokenForResolve(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw[0] == '"' || raw[0] == '\'' {
		q := raw[0]
		raw = raw[1:]
		if len(raw) > 0 && raw[len(raw)-1] == q {
			raw = raw[:len(raw)-1]
		}
	}
	s := unescapeShellPath(raw)
	if strings.Contains(s, "$") {
		exp, ok := expandEnvInPathToken(s)
		if !ok {
			return ""
		}
		s = exp
	}
	return normalizePathToken(s)
}

func expandEnvInPathToken(s string) (string, bool) {
	ok := true
	out := pathEnvVarRe.ReplaceAllStringFunc(s, func(m string) string {
		var name string
		if strings.HasPrefix(m, "${") {
			name = m[2 : len(m)-1]
		} else {
			name = m[1:]
		}
		v, found := os.LookupEnv(name)
		if !found {
			ok = false
			return m
		}
		return v
	})
	if !ok {
		return "", false
	}
	return out, true
}

func unescapeShellPath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case ' ', '\\', '"', '\'':
				b.WriteByte(s[i+1])
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func normalizePathToken(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\\", "/")
	if len(s) > 1 && strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

func expandTildePath(token string) (string, bool) {
	if token == "" || token[0] != '~' {
		return token, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	if token == "~" {
		return home, true
	}
	if len(token) > 1 && (token[1] == '/' || token[1] == '\\') {
		return filepath.Clean(filepath.Join(home, token[2:])), true
	}
	return "", false
}

func resolvePathForCompletion(projRoot, token string) (searchDir, base string, ok bool) {
	token = normalizePathToken(strings.TrimSpace(token))
	if token == "" {
		if strings.TrimSpace(projRoot) != "" {
			return resolvePathInsideRoot(projRoot, "")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", false
		}
		return home, "", true
	}
	if strings.HasPrefix(token, "~") {
		expanded, ok := expandTildePath(token)
		if !ok {
			return "", "", false
		}
		return splitSearchDirBase(expanded)
	}
	if filepath.IsAbs(token) {
		return splitSearchDirBase(filepath.Clean(token))
	}
	return resolvePathInsideRoot(projRoot, token)
}

func splitSearchDirBase(absPath string) (searchDir, base string, ok bool) {
	dir, base := filepath.Split(filepath.Clean(absPath))
	if dir == "" {
		return absPath, base, true
	}
	return dir, base, true
}

func resolvePathInsideRoot(projRoot, token string) (searchDir, base string, ok bool) {
	if strings.TrimSpace(projRoot) == "" {
		return "", "", false
	}
	absRoot, err := filepath.Abs(projRoot)
	if err != nil {
		return "", "", false
	}
	token = normalizePathToken(strings.TrimSpace(token))
	if token == "" {
		return absRoot, "", true
	}
	if filepath.IsAbs(token) {
		return "", "", false
	}
	cleaned := filepath.Clean(filepath.FromSlash(token))
	dir, base := filepath.Split(cleaned)
	joined := filepath.Clean(filepath.Join(absRoot, dir))
	if joined != absRoot && !strings.HasPrefix(joined, absRoot+string(filepath.Separator)) {
		return "", "", false
	}
	return joined, base, true
}

func (c *replCompleter) completePathToken(line []rune, pos, contentStart int) ([][]rune, int) {
	shell := line[contentStart:pos]
	span, ok := pathCompletionSpanInShell(shell, len(shell))
	if !ok {
		return nil, 0
	}
	replaceStart := contentStart + span.replaceStart
	searchDir, base, ok := resolvePathForCompletion(c.env.ProjRoot, span.pathPrefix)
	if !ok {
		return nil, 0
	}
	suffixes, err := pathEntrySuffixes(searchDir, base, span.inQuotes)
	if err != nil || len(suffixes) == 0 {
		return nil, 0
	}
	return completePathSuffixes(line, pos, replaceStart, suffixes)
}

func completePathSuffixes(line []rune, pos, wordStart int, suffixes []string) ([][]rune, int) {
	if wordStart > pos || len(suffixes) == 0 {
		return nil, 0
	}
	out := make([][]rune, 0, len(suffixes))
	for _, s := range suffixes {
		out = append(out, []rune(s))
	}
	return out, completeWordOffset(pos, wordStart)
}

func pathEntrySuffixes(searchDir, base string, inQuotes bool) ([]string, error) {
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		n := matchNamePrefixLen(name, base)
		if n < 0 {
			continue
		}
		suf := name[n:]
		if e.IsDir() {
			suf += string(filepath.Separator)
		}
		if !inQuotes {
			suf = escapePathCompletionSuffix(suf)
		}
		out = append(out, suf)
	}
	return out, nil
}

func escapePathCompletionSuffix(suffix string) string {
	if suffix == "" {
		return suffix
	}
	var b strings.Builder
	b.Grow(len(suffix) * 2)
	for i := 0; i < len(suffix); i++ {
		c := suffix[i]
		if needsShellEscapeByte(c) {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

func needsShellEscapeByte(c byte) bool {
	switch c {
	case ' ', '\t', '"', '\'', '\\', '$', '`', '!', '#', '&', '|', ';', '(', ')', '<', '>', '*', '?', '[', ']', '{', '}', '~':
		return true
	default:
		return false
	}
}

func matchNamePrefixLen(name, prefix string) int {
	if prefix == "" {
		return 0
	}
	if strings.HasPrefix(name, prefix) {
		return len(prefix)
	}
	if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
		return len(prefix)
	}
	return -1
}

