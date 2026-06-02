package agentruntime

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type shellCompleteKind int

const (
	shellCompleteNone shellCompleteKind = iota
	shellCompleteCommand
	shellCompleteGoSubcommand
	shellCompletePath
)

type shellCompleteCtx struct {
	kind        shellCompleteKind
	tokenStart  int
	tokenPrefix string
}

func analyzeShellAtPos(shell []rune, cursor int) shellCompleteCtx {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(shell) {
		cursor = len(shell)
	}
	tokenStart, tokenEnd := shellTokenBounds(shell, cursor)
	prefix := string(shell[tokenStart:tokenEnd])
	segWords := segmentWordsAtCursor(shell, cursor)
	wordIdx := wordIndexInSegment(segWords, tokenStart)
	if wordIdx == 0 {
		if looksLikePathToken(prefix) && prefix != "" {
			return shellCompleteCtx{kind: shellCompletePath, tokenStart: tokenStart, tokenPrefix: prefix}
		}
		return shellCompleteCtx{kind: shellCompleteCommand, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	if wordIdx == 1 && len(segWords) > 0 && isGoCommandName(segWords[0].text) {
		return shellCompleteCtx{kind: shellCompleteGoSubcommand, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	if looksLikePathToken(prefix) || wordIdx > 0 {
		return shellCompleteCtx{kind: shellCompletePath, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	return shellCompleteCtx{kind: shellCompleteNone, tokenStart: tokenStart, tokenPrefix: prefix}
}

func shellTokenBounds(shell []rune, cursor int) (start, end int) {
	if cursor > 0 && (shell[cursor-1] == ' ' || shell[cursor-1] == '\t') {
		start = cursor
		for start < len(shell) && (shell[start] == ' ' || shell[start] == '\t') {
			start++
		}
		return start, cursor
	}
	end = cursor
	for end > 0 && (shell[end-1] == ' ' || shell[end-1] == '\t') {
		end--
	}
	if end == 0 {
		return 0, 0
	}
	start = end - 1
	inQuote := false
	var quote rune
	for start >= 0 {
		ch := shell[start]
		if inQuote {
			if ch == quote && (start == 0 || shell[start-1] != '\\') {
				inQuote = false
				start--
				continue
			}
			if ch == '\\' && start > 0 {
				start -= 2
				continue
			}
			start--
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = true
			quote = ch
			start--
			continue
		}
		if ch == ' ' || ch == '\t' || isShellOpAt(shell, start) {
			break
		}
		start--
	}
	start++
	if start < 0 {
		start = 0
	}
	return start, cursor
}

type shellWord struct {
	start int
	end   int
	text  string
}

func segmentWordsAtCursor(shell []rune, cursor int) []shellWord {
	segStart := segmentStart(shell, cursor)
	var words []shellWord
	i := segStart
	for i < cursor {
		for i < cursor && (shell[i] == ' ' || shell[i] == '\t') {
			i++
		}
		if i >= cursor {
			break
		}
		if isShellOpAt(shell, i) {
			i = skipShellOp(shell, i)
			continue
		}
		wStart := i
		i = scanWordEnd(shell, i, cursor)
		text := string(shell[wStart:i])
		words = append(words, shellWord{start: wStart, end: i, text: text})
	}
	return words
}

func segmentStart(shell []rune, cursor int) int {
	i := cursor - 1
	for i >= 0 {
		if isShellOpAt(shell, i) {
			return skipShellOp(shell, i)
		}
		i--
	}
	return 0
}

func wordIndexInSegment(words []shellWord, tokenStart int) int {
	for i, w := range words {
		if tokenStart >= w.start && tokenStart < w.end {
			return i
		}
		if tokenStart == w.end {
			return i + 1
		}
	}
	return len(words)
}

func scanWordEnd(shell []rune, i, limit int) int {
	inQuote := false
	var quote rune
	for i < limit {
		ch := shell[i]
		if inQuote {
			if ch == quote && (i == 0 || shell[i-1] != '\\') {
				inQuote = false
			}
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = true
			quote = ch
			i++
			continue
		}
		if ch == ' ' || ch == '\t' || isShellOpAt(shell, i) {
			break
		}
		i++
	}
	return i
}

func isShellOpAt(shell []rune, i int) bool {
	if i < 0 || i >= len(shell) {
		return false
	}
	switch shell[i] {
	case '|':
		return true
	case ';':
		return true
	case '&':
		return i+1 < len(shell) && shell[i+1] == '&'
	default:
		return false
	}
}

func skipShellOp(shell []rune, i int) int {
	if i >= len(shell) {
		return i
	}
	switch shell[i] {
	case '|':
		if i+1 < len(shell) && shell[i+1] == '|' {
			return i + 2
		}
		return i + 1
	case ';':
		return i + 1
	case '&':
		if i+1 < len(shell) && shell[i+1] == '&' {
			return i + 2
		}
		return i + 1
	default:
		return i + 1
	}
}

func looksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, ".") {
		return true
	}
	if strings.Contains(token, "/") || strings.Contains(token, "\\") {
		return true
	}
	return false
}

func isGoCommandName(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	return name == "go"
}

func pathBinCandidates(prefix string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for name := range shellBuiltinsMap() {
		if matchBinPrefix(name, prefix) {
			add(name)
		}
	}
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return out
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !isPathExecutableFile(name) {
				continue
			}
			stem := executableStem(name)
			if stem == "" {
				continue
			}
			if matchBinPrefix(stem, prefix) {
				add(stem)
			}
		}
	}
	return out
}

func matchBinPrefix(name, prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

func isPathExecutableFile(name string) bool {
	if name == "" || name[0] == '.' {
		return false
	}
	if runtime.GOOS != "windows" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	for _, pe := range patExts() {
		if ext == strings.ToLower(pe) {
			return true
		}
	}
	return false
}

func patExts() []string {
	raw := os.Getenv("PATHEXT")
	if raw == "" {
		return []string{".COM", ".EXE", ".BAT", ".CMD"}
	}
	var exts []string
	for _, p := range strings.Split(raw, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p[0] != '.' {
			p = "." + p
		}
		exts = append(exts, p)
	}
	return exts
}

func executableStem(name string) string {
	if runtime.GOOS != "windows" {
		return name
	}
	lower := strings.ToLower(name)
	for _, pe := range patExts() {
		peLower := strings.ToLower(pe)
		if strings.HasSuffix(lower, peLower) {
			return name[:len(name)-len(pe)]
		}
	}
	return name
}

func shellBuiltinsMap() map[string]struct{} {
	out := make(map[string]struct{})
	shell := strings.ToLower(os.Getenv("SHELL"))
	comspec := strings.ToLower(os.Getenv("ComSpec"))
	switch {
	case strings.Contains(shell, "fish"):
		for _, n := range []string{"cd", "pwd", "export", "set"} {
			out[n] = struct{}{}
		}
	case strings.Contains(comspec, "cmd.exe") || comspec == "cmd":
		for _, n := range []string{"cd", "dir", "echo", "set"} {
			out[n] = struct{}{}
		}
	case strings.Contains(shell, "powershell") || strings.HasSuffix(shell, "pwsh"):
		for _, n := range []string{"cd", "dir", "ls", "pwd"} {
			out[n] = struct{}{}
		}
	default:
		for _, n := range []string{"alias", "bg", "cd", "echo", "export", "fg", "jobs", "pwd", "source", "type", "unset"} {
			out[n] = struct{}{}
		}
	}
	return out
}

func (c *replCompleter) completeShellLine(line []rune, pos, trimLeft int) ([][]rune, int, bool) {
	shellStart := trimLeft
	if line[shellStart] == '!' {
		shellStart++
		for shellStart < pos && (line[shellStart] == ' ' || line[shellStart] == '\t') {
			shellStart++
		}
	} else if !c.env.ReplShellFirst {
		return nil, 0, false
	}
	if shellStart >= pos {
		return nil, 0, false
	}
	shell := line[shellStart:pos]
	cursor := len(shell)
	ctx := analyzeShellAtPos(shell, cursor)
	absTokenStart := shellStart + ctx.tokenStart
	switch ctx.kind {
	case shellCompleteCommand:
		cands := pathBinCandidates(ctx.tokenPrefix)
		if len(cands) == 0 {
			return nil, 0, false
		}
		suf, off := completeCandidates(line, pos, absTokenStart, ctx.tokenPrefix, cands)
		if suf == nil {
			return nil, 0, false
		}
		return suf, off, true
	case shellCompleteGoSubcommand:
		cands := goSubcommandCandidates()
		if len(cands) == 0 {
			return nil, 0, false
		}
		suf, off := completeCandidates(line, pos, absTokenStart, ctx.tokenPrefix, cands)
		if suf == nil {
			return nil, 0, false
		}
		return suf, off, true
	case shellCompletePath:
		suf, off := c.completePathToken(line, pos, shellStart)
		if suf == nil {
			return nil, 0, false
		}
		return suf, off, true
	default:
		return nil, 0, false
	}
}
