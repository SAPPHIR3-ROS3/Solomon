package replcomplete

import (
	"strings"
	"unicode"
)

func SlashSuggest(env ReplCompleteEnv, buffer string, slashHistory []string) string {
	trimLeft := 0
	for trimLeft < len(buffer) && (buffer[trimLeft] == ' ' || buffer[trimLeft] == '\t') {
		trimLeft++
	}
	if trimLeft >= len(buffer) || buffer[trimLeft] != '/' {
		return ""
	}
	leading := buffer[:trimLeft]
	c := &replCompleter{env: env}
	trimmed := buffer[trimLeft:]
	rest := trimmed[1:]
	sp := strings.Index(rest, " ")
	if sp < 0 {
		prefix := strings.ToLower(rest)
		var matches []string
		for _, name := range c.slashCommandNames() {
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, "/"+name)
			}
		}
		return leading + pickSlashSuggestion(buffer, matches, slashHistory)
	}
	cmd := strings.ToLower(strings.TrimSpace(rest[:sp]))
	argStart := 1 + sp + 1
	for argStart < len(trimmed) && (trimmed[argStart] == ' ' || trimmed[argStart] == '\t') {
		argStart++
	}
	if argStart > len(trimmed) {
		return ""
	}
	argPrefix := strings.ToLower(trimmed[argStart:])
	candidates := slashStaticArgCandidates(cmd)
	if candidates == nil {
		switch cmd {
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
	head := trimmed[:argStart]
	var matches []string
	for _, arg := range candidates {
		if !strings.HasPrefix(strings.ToLower(arg), argPrefix) {
			continue
		}
		matches = append(matches, head+arg)
	}
	return leading + pickSlashSuggestion(buffer, matches, slashHistory)
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
