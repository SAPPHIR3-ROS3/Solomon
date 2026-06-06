package replhl

import (
	"strings"
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func HighlightSlash(line string, env replcomplete.ReplCompleteEnv) string {
	if line == "" {
		return ""
	}
	rs := []rune(line)
	styles := make([]termcolor.ZshStyleKey, len(rs))
	applySlashLexical(rs, styles)
	applySlashSemantic(rs, styles, env)
	return renderStyled(line, styles)
}

func applySlashLexical(rs []rune, styles []termcolor.ZshStyleKey) {
	inSingle := false
	inDouble := false
	for i := 0; i < len(rs); i++ {
		ch := rs[i]
		if inSingle {
			forceSpan(styles, i, i+1, termcolor.ZshSingleQuoted)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '$' {
				end := dollarVarEnd(rs, i)
				forceSpan(styles, i, end, termcolor.ZshDollarDoubleQuoted)
				i = end - 1
				continue
			}
			forceSpan(styles, i, i+1, termcolor.ZshDoubleQuoted)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			forceSpan(styles, i, i+1, termcolor.ZshSingleQuoted)
			continue
		}
		if ch == '"' {
			inDouble = true
			forceSpan(styles, i, i+1, termcolor.ZshDoubleQuoted)
			continue
		}
		if ch == '$' {
			end := dollarVarEnd(rs, i)
			forceSpan(styles, i, end, termcolor.ZshDollarDoubleQuoted)
			i = end - 1
		}
	}
}

func applySlashSemantic(rs []rune, styles []termcolor.ZshStyleKey, env replcomplete.ReplCompleteEnv) {
	trim := strings.TrimLeft(string(rs), " \t")
	if trim == "" || trim[0] != '/' {
		return
	}
	lead := len(string(rs)) - len(trim)
	cmdStart := lead + 1
	i := cmdStart
	for i < len(rs) && rs[i] != ' ' && rs[i] != '\t' {
		i++
	}
	if i <= cmdStart {
		return
	}
	cmd := strings.ToLower(string(rs[cmdStart:i]))
	key := termcolor.ZshUnknownToken
	if replcomplete.SlashCommandKnown(env, cmd) {
		key = termcolor.ZshArg0
	}
	forceSpan(styles, cmdStart, i, key)
	for argPos := i; argPos < len(rs); {
		for argPos < len(rs) && (rs[argPos] == ' ' || rs[argPos] == '\t') {
			argPos++
		}
		if argPos >= len(rs) {
			break
		}
		if rs[argPos] == '"' || rs[argPos] == '\'' {
			q := rs[argPos]
			end := argPos + 1
			for end < len(rs) && rs[end] != q {
				end++
			}
			if end < len(rs) {
				end++
			}
			argPos = end
			continue
		}
		end := argPos
		for end < len(rs) && rs[end] != ' ' && rs[end] != '\t' {
			end++
		}
		token := string(rs[argPos:end])
		if looksSlashPathArg(token) {
			exists, isPrefix := replcomplete.PathHighlightStatus(env.ProjRoot, token)
			if exists {
				forceSpan(styles, argPos, end, termcolor.ZshPath)
			} else if isPrefix {
				forceSpan(styles, argPos, end, termcolor.ZshPathPrefix)
			}
		}
		argPos = end
	}
}

func looksSlashPathArg(token string) bool {
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, "/") || strings.HasPrefix(token, ".") || strings.HasPrefix(token, "~") {
		return true
	}
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return strings.Contains(token, "/") || strings.Contains(token, "\\")
	}
	return false
}
