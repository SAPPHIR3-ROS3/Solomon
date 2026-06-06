package replhl

import (
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shelllex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type ShellEnv struct {
	ProjRoot string
}

func HighlightShell(line string, env ShellEnv) string {
	if line == "" {
		return ""
	}
	rs := []rune(line)
	styles := make([]termcolor.ZshStyleKey, len(rs))
	applyShellLexical(rs, styles)
	applyShellSemantic(rs, styles, env)
	return renderStyled(line, styles)
}

func applyShellLexical(rs []rune, styles []termcolor.ZshStyleKey) {
	inSingle := false
	inDouble := false
	comment := false
	for i := 0; i < len(rs); i++ {
		if comment {
			forceSpan(styles, i, len(rs), termcolor.ZshComment)
			break
		}
		ch := rs[i]
		if inSingle {
			forceSpan(styles, i, i+1, termcolor.ZshSingleQuoted)
			if ch == '\'' && !shelllex.IsEscapedAt(rs, i) {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '$' && !shelllex.IsEscapedAt(rs, i) {
				end := dollarVarEnd(rs, i)
				forceSpan(styles, i, end, termcolor.ZshDollarDoubleQuoted)
				i = end - 1
				continue
			}
			forceSpan(styles, i, i+1, termcolor.ZshDoubleQuoted)
			if ch == '"' && !shelllex.IsEscapedAt(rs, i) {
				inDouble = false
			}
			continue
		}
		if ch == '#' && !shelllex.IsEscapedAt(rs, i) {
			comment = true
			forceSpan(styles, i, len(rs), termcolor.ZshComment)
			break
		}
		if ch == '\'' && !shelllex.IsEscapedAt(rs, i) {
			inSingle = true
			forceSpan(styles, i, i+1, termcolor.ZshSingleQuoted)
			continue
		}
		if ch == '"' && !shelllex.IsEscapedAt(rs, i) {
			inDouble = true
			forceSpan(styles, i, i+1, termcolor.ZshDoubleQuoted)
			continue
		}
		if shelllex.IsShellOpAt(rs, i) {
			end := shelllex.SkipShellOp(rs, i)
			forceSpan(styles, i, end, termcolor.ZshCommandSeparator)
			i = end - 1
			continue
		}
		if isRedirectAt(rs, i) {
			end := redirectEnd(rs, i)
			forceSpan(styles, i, end, termcolor.ZshRedirection)
			i = end - 1
			continue
		}
		if ch == '$' && !shelllex.IsEscapedAt(rs, i) {
			end := dollarVarEnd(rs, i)
			forceSpan(styles, i, end, termcolor.ZshDollarDoubleQuoted)
			i = end - 1
			continue
		}
		if isGlobChar(ch) {
			forceSpan(styles, i, i+1, termcolor.ZshGlobbing)
		}
	}
}

func applyShellSemantic(rs []rune, styles []termcolor.ZshStyleKey, env ShellEnv) {
	segs := shelllex.Segments(rs)
	for _, seg := range segs {
		for wi, w := range seg.Words {
			if w.Start >= w.End {
				continue
			}
			if wordHasQuoteStyle(styles, w.Start, w.End) {
				continue
			}
			text := w.Text
			if wi == 0 || (wi == 1 && len(seg.Words) > 0 && shelllex.IsGoCommandName(seg.Words[0].Text)) {
				name := shelllex.CommandNameFromWord(text)
				if name == "" {
					continue
				}
				found, isBuiltin := shelllex.CommandKnown(name)
				key := termcolor.ZshUnknownToken
				if found {
					if isBuiltin {
						key = termcolor.ZshBuiltin
					} else {
						key = termcolor.ZshArg0
					}
				}
				forceSpan(styles, w.Start, w.End, key)
				continue
			}
			if shelllex.LooksLikePathToken(text) {
				exists, isPrefix := replcomplete.PathHighlightStatus(env.ProjRoot, text)
				if exists {
					forceSpan(styles, w.Start, w.End, termcolor.ZshPath)
				} else if isPrefix {
					forceSpan(styles, w.Start, w.End, termcolor.ZshPathPrefix)
				}
			}
		}
	}
}

func wordHasQuoteStyle(styles []termcolor.ZshStyleKey, start, end int) bool {
	for i := start; i < end && i < len(styles); i++ {
		switch styles[i] {
		case termcolor.ZshSingleQuoted, termcolor.ZshDoubleQuoted, termcolor.ZshComment:
			return true
		}
	}
	return false
}

func isRedirectAt(rs []rune, i int) bool {
	if i >= len(rs) {
		return false
	}
	switch rs[i] {
	case '<', '>':
		return true
	}
	if rs[i] >= '0' && rs[i] <= '9' {
		j := i
		for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
			j++
		}
		if j < len(rs) && rs[j] == '>' {
			return true
		}
	}
	return false
}

func redirectEnd(rs []rune, i int) int {
	if i >= len(rs) {
		return i
	}
	j := i
	if rs[j] >= '0' && rs[j] <= '9' {
		for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
			j++
		}
	}
	if j < len(rs) && (rs[j] == '>' || rs[j] == '<') {
		j++
		if j < len(rs) && rs[j] == rs[j-1] {
			j++
		}
		return j
	}
	if rs[i] == '>' || rs[i] == '<' {
		j = i + 1
		if j < len(rs) && rs[j] == rs[i] {
			j++
		}
		return j
	}
	return i + 1
}

func dollarVarEnd(rs []rune, i int) int {
	if i >= len(rs) || rs[i] != '$' {
		return i + 1
	}
	if i+1 < len(rs) && rs[i+1] == '{' {
		j := i + 2
		for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
			j++
		}
		if j < len(rs) && rs[j] == '}' {
			return j + 1
		}
		return j
	}
	j := i + 1
	if j < len(rs) && (unicode.IsLetter(rs[j]) || rs[j] == '_') {
		j++
		for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
			j++
		}
	}
	if j == i+1 {
		return i + 1
	}
	return j
}

func isGlobChar(ch rune) bool {
	switch ch {
	case '*', '?', '[', ']':
		return true
	default:
		return false
	}
}

func HighlightShellBufferLine(fullLine string, shellText string, byteOffset int, env ShellEnv) string {
	if shellText == "" {
		return fullLine
	}
	hl := HighlightShell(shellText, env)
	if byteOffset <= 0 {
		return hl
	}
	if byteOffset >= len(fullLine) {
		return fullLine
	}
	return fullLine[:byteOffset] + hl
}
