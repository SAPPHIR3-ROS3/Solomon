package agentruntime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"

	readline "github.com/chzyer/readline"
)

type ReplCompleteEnv struct {
	ProjHex        string
	ProjRoot       string
	ReplShellFirst bool
	Session        func() *chatstore.Session
}

type replCompleter struct {
	env ReplCompleteEnv
}

func ReplCompletionDisabled() bool {
	return os.Getenv("SOLOMON_NO_COMPLETE") != ""
}

func NewReplCompleter(env ReplCompleteEnv) readline.AutoCompleter {
	if ReplCompletionDisabled() {
		return nil
	}
	return &replCompleter{env: env}
}

func ReplCompleteDo(env ReplCompleteEnv, line []rune, pos int) ([][]rune, int) {
	c := &replCompleter{env: env}
	return c.Do(line, pos)
}

func (c *replCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(line) {
		pos = len(line)
	}
	head := line[:pos]
	trimLeft := 0
	for trimLeft < len(head) && (head[trimLeft] == ' ' || head[trimLeft] == '\t') {
		trimLeft++
	}
	if trimLeft >= len(head) {
		return nil, 0
	}
	if head[trimLeft] == '/' {
		return c.completeSlash(head, pos, trimLeft)
	}
	if suf, off, ok := c.completeShellLine(head, pos, trimLeft); ok {
		return suf, off
	}
	return nil, 0
}

func (c *replCompleter) completeSlash(line []rune, pos, trimLeft int) ([][]rune, int) {
	text := string(line[:pos])
	trimmed := string(line[trimLeft:pos])
	slashOff := len(text) - len(trimmed)
	rest := trimmed[1:]
	sp := strings.Index(rest, " ")
	if sp < 0 {
		prefix := strings.ToLower(rest)
		start := slashOff + 1
		return completeCandidates(line, pos, start, prefix, c.slashCommandNames())
	}
	cmd := strings.ToLower(strings.TrimSpace(rest[:sp]))
	argStart := slashOff + 1 + sp + 1
	for argStart < pos && (line[argStart] == ' ' || line[argStart] == '\t') {
		argStart++
	}
	if argStart > pos {
		return nil, 0
	}
	argPrefix := strings.ToLower(string(line[argStart:pos]))
	return c.completeArg(cmd, line, pos, argStart, argPrefix)
}


func (c *replCompleter) completePathToken(line []rune, pos, contentStart int) ([][]rune, int) {
	shell := line[contentStart:pos]
	tokenOff := lastShellTokenOffset(shell)
	tokenStart := contentStart + tokenOff
	prefix := string(line[tokenStart:pos])
	searchDir, base, ok := resolvePathInsideRoot(c.env.ProjRoot, prefix)
	if !ok {
		return nil, 0
	}
	suffixes, err := pathEntrySuffixes(searchDir, base)
	if err != nil || len(suffixes) == 0 {
		return nil, 0
	}
	return completePathSuffixes(line, pos, tokenStart, suffixes)
}

func lastShellTokenOffset(shell []rune) int {
	end := len(shell)
	for end > 0 && (shell[end-1] == ' ' || shell[end-1] == '\t') {
		end--
	}
	if end == 0 {
		return 0
	}
	for i := end - 1; i >= 0; i-- {
		if shell[i] == ' ' || shell[i] == '\t' {
			return i + 1
		}
	}
	return 0
}

func resolvePathInsideRoot(projRoot, token string) (searchDir, base string, ok bool) {
	if strings.TrimSpace(projRoot) == "" {
		return "", "", false
	}
	absRoot, err := filepath.Abs(projRoot)
	if err != nil {
		return "", "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return absRoot, "", true
	}
	if filepath.IsAbs(token) {
		return "", "", false
	}
	dir, base := filepath.Split(filepath.Clean(token))
	joined := filepath.Clean(filepath.Join(absRoot, dir))
	if joined != absRoot && !strings.HasPrefix(joined, absRoot+string(filepath.Separator)) {
		return "", "", false
	}
	return joined, base, true
}

func pathEntrySuffixes(searchDir, base string) ([]string, error) {
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
		out = append(out, suf)
	}
	return out, nil
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

func completePathSuffixes(line []rune, pos, startIdx int, suffixes []string) ([][]rune, int) {
	if startIdx > pos || len(suffixes) == 0 {
		return nil, 0
	}
	out := make([][]rune, 0, len(suffixes))
	for _, s := range suffixes {
		out = append(out, []rune(s))
	}
	return out, startIdx
}

func (c *replCompleter) slashCommandNames() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, n := range commands.SlashBuiltinNames() {
		add(n)
	}
	if c.env.ProjHex != "" || c.env.ProjRoot != "" {
		skillNames, err := skills.InstalledSlashCommandNames(c.env.ProjHex, c.env.ProjRoot)
		if err == nil {
			for _, n := range skillNames {
				add(n)
			}
		}
	}
	return out
}

func (c *replCompleter) completeArg(cmd string, line []rune, pos, argStart int, argPrefix string) ([][]rune, int) {
	candidates := slashStaticArgCandidates(cmd)
	if candidates == nil {
		switch cmd {
		case "resume":
			candidates = c.resumeCandidates()
		case "goto":
			candidates = c.gotoCandidates()
		default:
			return nil, 0
		}
	}
	return completeCandidates(line, pos, argStart, argPrefix, candidates)
}

func (c *replCompleter) resumeCandidates() []string {
	out := []string{"last"}
	if c.env.ProjHex == "" {
		return out
	}
	list, err := chatstore.ListRecent(c.env.ProjHex, 10)
	if err != nil {
		return out
	}
	seen := map[string]struct{}{"last": {}}
	for _, s := range list {
		if s == nil {
			continue
		}
		if s.ID != "" {
			if _, ok := seen[s.ID]; !ok {
				seen[s.ID] = struct{}{}
				out = append(out, s.ID)
			}
		}
		if t := strings.TrimSpace(s.Title); t != "" {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
			}
		}
	}
	return out
}

func (c *replCompleter) gotoCandidates() []string {
	sess := c.env.Session()
	if sess == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(seq int, suffix string) {
		tag := checkpoint.FormatCheckpointTag(seq, suffix)
		tag = strings.Trim(tag, "[]")
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
		if n := strconv.Itoa(seq); n != "" {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				out = append(out, n)
			}
		}
	}
	for _, m := range sess.Messages {
		if m.CpSeqSet || m.CheckpointSeq >= 0 {
			add(m.CheckpointSeq, m.CheckpointBranchKey)
		}
	}
	for i := 0; i <= sess.CheckpointLast; i++ {
		add(i, "")
		add(i, sess.CheckpointBranchSuffix)
	}
	return out
}

func (r *Runtime) replCompleteEnv() ReplCompleteEnv {
	return ReplCompleteEnv{
		ProjHex:        r.ProjHex,
		ProjRoot:       r.ProjRoot,
		ReplShellFirst: r.ReplShellFirst,
		Session:        r.snapshotSession,
	}
}

func (r *Runtime) snapshotSession() *chatstore.Session {
	var snap *chatstore.Session
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		cp := *s
		snap = &cp
	})
	return snap
}

func completeCandidates(line []rune, pos, startIdx int, prefix string, candidates []string) ([][]rune, int) {
	if startIdx > pos {
		return nil, 0
	}
	prefix = strings.ToLower(prefix)
	var suffixes [][]rune
	for _, cand := range candidates {
		cl := strings.ToLower(cand)
		if !strings.HasPrefix(cl, prefix) {
			continue
		}
		suffixes = append(suffixes, []rune(cand[len(prefix):]))
	}
	if len(suffixes) == 0 {
		return nil, 0
	}
	return suffixes, startIdx
}
