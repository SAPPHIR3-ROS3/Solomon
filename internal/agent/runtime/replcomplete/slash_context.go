package replcomplete

import (
	"strings"
)

type SlashContext struct {
	Active     bool
	SlashStart int
	CmdStart   int
	ArgStart   int
	Cmd        string
}

func SlashContextAt(line []rune, col int) (SlashContext, bool) {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	slashStart := -1
	for i := col - 1; i >= 0; i-- {
		if line[i] == '/' {
			slashStart = i
			break
		}
	}
	if slashStart < 0 {
		return SlashContext{}, false
	}
	if slashStart > 0 {
		prev := line[slashStart-1]
		if prev != ' ' && prev != '\t' && prev != '\n' {
			return SlashContext{}, false
		}
	}
	rest := string(line[slashStart+1 : col])
	sp := strings.Index(rest, " ")
	if sp < 0 {
		return SlashContext{
			Active:     true,
			SlashStart: slashStart,
			CmdStart:   slashStart + 1,
			ArgStart:   -1,
		}, true
	}
	cmd := strings.TrimSpace(rest[:sp])
	argStart := slashStart + 1 + sp
	for argStart < col && (line[argStart] == ' ' || line[argStart] == '\t') {
		argStart++
	}
	return SlashContext{
		Active:     true,
		SlashStart: slashStart,
		CmdStart:   slashStart + 1,
		ArgStart:   argStart,
		Cmd:        strings.ToLower(cmd),
	}, true
}

type SlashToken struct {
	SlashStart int
	CmdStart   int
	CmdEnd     int
	ArgStart   int
	ArgEnd     int
}

func slashBoundaryBefore(line []rune, slashIdx int) bool {
	if slashIdx <= 0 {
		return true
	}
	prev := line[slashIdx-1]
	return prev == ' ' || prev == '\t' || prev == '\n'
}

func SlashTokensInLine(line []rune) []SlashToken {
	inSingle := false
	inDouble := false
	var tokens []SlashToken
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if ch != '/' || !slashBoundaryBefore(line, i) {
			continue
		}
		cmdStart := i + 1
		cmdEnd := cmdStart
		for cmdEnd < len(line) && line[cmdEnd] != ' ' && line[cmdEnd] != '\t' {
			cmdEnd++
		}
		if cmdEnd <= cmdStart {
			continue
		}
		argStart := cmdEnd
		for argStart < len(line) && (line[argStart] == ' ' || line[argStart] == '\t') {
			argStart++
		}
		argEnd := len(line)
		for j := argStart + 1; j < len(line); j++ {
			if line[j] == '/' && slashBoundaryBefore(line, j) {
				argEnd = j
				for argEnd > argStart && (line[argEnd-1] == ' ' || line[argEnd-1] == '\t') {
					argEnd--
				}
				break
			}
		}
		tokens = append(tokens, SlashToken{
			SlashStart: i,
			CmdStart:   cmdStart,
			CmdEnd:     cmdEnd,
			ArgStart:   argStart,
			ArgEnd:     argEnd,
		})
	}
	return tokens
}
