package editor

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
)

func (e *multilineEditor) recomputeAtPicker() {
	e.atMatches = nil
	e.atSelected = 0
	e.atCtx = atmention.AtContext{}
	line := e.lines[e.row]
	ctx := atmention.AtContextAt(line, e.col)
	if !ctx.Active {
		return
	}
	e.atCtx = ctx
	entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
	if err != nil || len(entries) == 0 {
		return
	}
	e.atMatches = atmention.MatchQuery(ctx.Query, entries, atmention.MaxPickerResults)
}

func (e *multilineEditor) atPickerItemVisible(i int) bool {
	if i < 0 || i >= len(e.atMatches) {
		return false
	}
	if len(e.suggestSuffix) > 0 && i == e.atSelected {
		return false
	}
	return true
}

func (e *multilineEditor) atPickerVisibleCount() int {
	n := 0
	for i := range e.atMatches {
		if e.atPickerItemVisible(i) {
			n++
		}
	}
	return n
}

func (e *multilineEditor) atPickerActive() bool {
	return e.atCtx.Active && len(e.atMatches) > 0 && e.atPickerVisibleCount() > 0
}

func (e *multilineEditor) atPickerUp() {
	if !e.atPickerActive() {
		return
	}
	if e.atSelected > 0 {
		e.atSelected--
	}
}

func (e *multilineEditor) atPickerDown() {
	if !e.atPickerActive() {
		return
	}
	if e.atSelected+1 < len(e.atMatches) {
		e.atSelected++
	}
}

func (e *multilineEditor) clearAtStateAfterInsert() {
	e.atMatches = nil
	e.atCtx = atmention.AtContext{}
	e.atSelected = 0
	e.clearSuggest()
}

func (e *multilineEditor) insertAtTag(sel atmention.Entry, withSpace bool) bool {
	entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
	if err != nil {
		return false
	}
	tag := "@" + atmention.ShortTag(sel.RelPath, entries)
	if withSpace {
		tag += " "
	}
	start := e.atCtx.ReplaceStart
	end := e.col
	line := e.lines[e.row]
	newLine := append(append(append([]rune(nil), line[:start]...), []rune(tag)...), line[end:]...)
	e.lines[e.row] = newLine
	e.col = start + len([]rune(tag))
	e.clearAtStateAfterInsert()
	return true
}

func (e *multilineEditor) completeAtMention() bool {
	line := e.lines[e.row]
	ctx := atmention.AtContextAt(line, e.col)
	if !ctx.Active {
		return false
	}
	e.atCtx = ctx

	if e.cursorAtBufferEnd() && len(e.suggestSuffix) > 0 {
		e.acceptSuggest(false)
		e.insertRune(' ')
		e.clearAtStateAfterInsert()
		return true
	}

	if len(e.atMatches) == 0 {
		entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
		if err != nil || len(entries) == 0 {
			return false
		}
		e.atMatches = atmention.MatchQuery(ctx.Query, entries, atmention.MaxPickerResults)
	}
	if len(e.atMatches) == 0 {
		return false
	}
	return e.insertAtTag(e.atMatches[e.atSelected], true)
}
