package replcomplete

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shelllex"
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
	tokenStart, tokenEnd := shelllex.ShellTokenBounds(shell, cursor)
	prefix := string(shell[tokenStart:tokenEnd])
	segWords := shelllex.SegmentWordsAtCursor(shell, cursor)
	wordIdx := shelllex.WordIndexInSegment(segWords, tokenStart)
	if wordIdx == 0 {
		if shelllex.LooksLikePathToken(prefix) && prefix != "" {
			return shellCompleteCtx{kind: shellCompletePath, tokenStart: tokenStart, tokenPrefix: prefix}
		}
		return shellCompleteCtx{kind: shellCompleteCommand, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	if wordIdx == 1 && len(segWords) > 0 && shelllex.IsGoCommandName(segWords[0].Text) {
		return shellCompleteCtx{kind: shellCompleteGoSubcommand, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	if shelllex.LooksLikePathToken(prefix) || wordIdx > 0 {
		return shellCompleteCtx{kind: shellCompletePath, tokenStart: tokenStart, tokenPrefix: prefix}
	}
	return shellCompleteCtx{kind: shellCompleteNone, tokenStart: tokenStart, tokenPrefix: prefix}
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
		cands := shelllex.PathBinCandidates(ctx.tokenPrefix)
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
