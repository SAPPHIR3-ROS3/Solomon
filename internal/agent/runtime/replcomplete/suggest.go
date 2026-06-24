package replcomplete

import (
	"strings"
	"unicode"
)

func SlashSuggest(env ReplCompleteEnv, buffer string, slashHistory []string) string {
	line := []rune(buffer)
	return SlashSuggestAt(line, len(line), env, slashHistory)
}

func SlashSuggestAt(line []rune, col int, env ReplCompleteEnv, slashHistory []string) string {
	ctx, ok := SlashContextAt(line, col)
	if !ok {
		return ""
	}
	prefix := string(line[:col])
	head := string(line[:ctx.SlashStart])
	c := &replCompleter{env: env}
	if ctx.ArgStart < 0 {
		rest := string(line[ctx.CmdStart:col])
		cmdPrefix := strings.ToLower(rest)
		var matches []string
		for _, name := range c.slashCommandNames() {
			if strings.HasPrefix(name, cmdPrefix) {
				matches = append(matches, head+"/"+name)
			}
		}
		return pickSlashSuggestion(prefix, matches, slashHistory)
	}
	candidates := slashStaticArgCandidates(ctx.Cmd)
	if candidates == nil {
		switch ctx.Cmd {
		case "resume":
			candidates = c.resumeCandidates()
		case "goto":
			candidates = c.gotoCandidates()
		case "rewind":
			candidates = c.rewindCandidates()
		default:
			return ""
		}
	}
	argPrefix := strings.ToLower(string(line[ctx.ArgStart:col]))
	argHead := string(line[:ctx.ArgStart])
	var matches []string
	for _, arg := range candidates {
		if !strings.HasPrefix(strings.ToLower(arg), argPrefix) {
			continue
		}
		matches = append(matches, argHead+arg)
	}
	return pickSlashSuggestion(prefix, matches, slashHistory)
}

func pickSlashSuggestion(buffer string, catalogMatches []string, slashHistory []string) string {
	if len(catalogMatches) == 0 {
		return ""
	}
	bufLower := strings.ToLower(buffer)
	if len(catalogMatches) == 1 {
		m := catalogMatches[0]
		if strings.HasPrefix(strings.ToLower(m), bufLower) && !strings.EqualFold(m, buffer) {
			return m
		}
		return ""
	}
	for i := len(slashHistory) - 1; i >= 0; i-- {
		h := slashHistory[i]
		hLower := strings.ToLower(h)
		if !strings.HasPrefix(hLower, bufLower) || strings.EqualFold(h, buffer) {
			continue
		}
		for _, m := range catalogMatches {
			mLower := strings.ToLower(m)
			if strings.HasPrefix(hLower, mLower) {
				return h
			}
		}
	}
	return ""
}

func SuggestSuffixFromFull(buffer, full string) string {
	br := []rune(buffer)
	fr := []rune(full)
	i := 0
	for i < len(br) && i < len(fr) && unicode.ToLower(br[i]) == unicode.ToLower(fr[i]) {
		i++
	}
	if i >= len(fr) || i < len(br) {
		return ""
	}
	return string(fr[i:])
}
