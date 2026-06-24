package editor

import (
	"context"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shellhist"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
)

func AutosuggestDisabled() bool {
	return os.Getenv("SOLOMON_NO_AUTOSUGGEST") != ""
}

func shellPrefixNormalized(buffer string, shellFirst bool) string {
	trimmed := strings.TrimLeft(buffer, " \t")
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return ""
	}
	if shellFirst {
		if strings.HasPrefix(trimmed, "!") {
			return ""
		}
		return multiline.TrimMessageEdges(trimmed)
	}
	if !strings.HasPrefix(trimmed, "!") {
		return ""
	}
	cmd := multiline.TrimMessageEdges(strings.TrimPrefix(trimmed, "!"))
	if cmd == "" {
		return ""
	}
	return cmd
}

func bufferHasImgTag(s string) bool {
	return strings.Contains(s, "[img-")
}

func firstWordRunes(rs []rune) []rune {
	if len(rs) == 0 {
		return nil
	}
	i := 0
	for i < len(rs) && rs[i] != ' ' && rs[i] != '\t' && rs[i] != '\n' {
		i++
	}
	if i == 0 && (rs[0] == ' ' || rs[0] == '\t') {
		for i < len(rs) && (rs[i] == ' ' || rs[i] == '\t') {
			i++
		}
		start := i
		for i < len(rs) && rs[i] != ' ' && rs[i] != '\t' && rs[i] != '\n' {
			i++
		}
		return rs[start:i]
	}
	return rs[:i]
}

func (e *multilineEditor) cursorAtBufferEnd() bool {
	if e.row != len(e.lines)-1 {
		return false
	}
	return e.col == len(e.lines[e.row])
}

func (e *multilineEditor) clearSuggest() {
	e.suggestSuffix = nil
}

func (e *multilineEditor) recomputeSuggest() {
	e.suggestSuffix = nil
	if AutosuggestDisabled() {
		return
	}
	line := e.lines[e.row]
	env := e.host.CompleteEnv
	if ctx, ok := replcomplete.SlashContextAt(line, e.col); ok && ctx.Active {
		buf := string(line[:e.col])
		full := replcomplete.SlashSuggestAt(line, e.col, env, e.history.slashLinesCopy())
		if suf := replcomplete.SuggestSuffixFromFull(buf, full); suf != "" {
			e.suggestSuffix = []rune(suf)
		}
		return
	}
	if !e.cursorAtBufferEnd() {
		return
	}
	buf := e.string()
	if strings.TrimSpace(buf) == "" || bufferHasImgTag(buf) {
		return
	}
	if e.atPickerActive() {
		entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
		if err == nil && len(entries) > 0 && len(e.atMatches) > 0 {
			sel := e.atMatches[e.atSelected]
			tag := "@" + atmention.ShortTag(sel.RelPath, entries)
			line := e.lines[e.row]
			if e.col <= len(line) && len(tag) > e.col-e.atCtx.ReplaceStart {
				suf := tag[e.col-e.atCtx.ReplaceStart:]
				if suf != "" {
					e.suggestSuffix = []rune(suf)
				}
			}
		}
		return
	}
	shellFirst := env.ReplShellFirst
	trimmed := strings.TrimLeft(buf, " \t")
	if strings.HasPrefix(trimmed, "/") {
		full := replcomplete.SlashSuggest(env, buf, e.history.slashLinesCopy())
		if suf := replcomplete.SuggestSuffixFromFull(buf, full); suf != "" {
			e.suggestSuffix = []rune(suf)
		}
		return
	}
	norm := shellPrefixNormalized(buf, shellFirst)
	if norm == "" {
		return
	}
	match := e.history.shellMatch(norm)
	if match == "" {
		match = shellhist.Suggest(norm)
	}
	if match == "" || match == norm {
		return
	}
	if suf := replcomplete.SuggestSuffixFromFull(norm, match); suf != "" {
		e.suggestSuffix = []rune(suf)
	}
}

func (e *multilineEditor) acceptSuggest(partialWord bool) {
	if len(e.suggestSuffix) == 0 {
		return
	}
	var insert []rune
	if partialWord {
		insert = firstWordRunes(e.suggestSuffix)
	} else {
		insert = append([]rune(nil), e.suggestSuffix...)
	}
	if len(insert) == 0 {
		return
	}
	for _, r := range insert {
		e.insertRuneRaw(r)
	}
	e.suggestSuffix = e.suggestSuffix[len(insert):]
	if len(e.suggestSuffix) > 0 {
		return
	}
	e.recomputeSuggest()
}
