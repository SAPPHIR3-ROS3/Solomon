package replcomplete

import (
	"os"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"

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
	if suf, off, ok := c.completeAtMention(head, pos); ok {
		return suf, off
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


func (c *replCompleter) slashCommandNames() []string {
	return SlashCommandNames(c.env)
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
	return suffixes, completeWordOffset(pos, startIdx)
}

func completeWordOffset(pos, wordStart int) int {
	if wordStart > pos {
		return 0
	}
	return pos - wordStart
}
