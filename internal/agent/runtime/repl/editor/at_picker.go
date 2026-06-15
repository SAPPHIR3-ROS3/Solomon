package editor

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
)

func (e *multilineEditor) recomputeAtPicker() {
	if e.atCycleActive {
		e.atMatches = e.atCycleMatches
		e.atSelected = e.atCycleIndex
		return
	}
	line := e.lines[e.row]
	ctx := atmention.AtContextAt(line, e.col)
	if !ctx.Active {
		e.atMatches = nil
		e.atCtx = atmention.AtContext{}
		e.atSelected = 0
		return
	}
	prevQuery := e.atCtx.Query
	prevStart := e.atCtx.ReplaceStart
	prevSel := e.atSelected
	e.atCtx = ctx
	entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
	if err != nil || len(entries) == 0 {
		e.atMatches = nil
		return
	}
	e.atMatches = atmention.MatchQuery(ctx.Query, entries, atmention.MaxPickerResults)
	if ctx.Query == prevQuery && ctx.ReplaceStart == prevStart && prevSel < len(e.atMatches) {
		e.atSelected = prevSel
	} else {
		e.atSelected = 0
	}
}

func (e *multilineEditor) atPickerActive() bool {
	return len(e.atMatches) > 0 && (e.atCtx.Active || e.atCycleActive)
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

func (e *multilineEditor) atPickerUp() {
	if len(e.atMatches) == 0 {
		return
	}
	if e.atCycleActive {
		if e.atCycleIndex > 0 {
			e.atCycleIndex--
		}
		e.applyAtCycleTag(e.atCycleMatches[e.atCycleIndex])
		return
	}
	if e.atSelected > 0 {
		e.atSelected--
	}
}

func (e *multilineEditor) atPickerDown() {
	if len(e.atMatches) == 0 {
		return
	}
	if e.atCycleActive {
		if e.atCycleIndex+1 < len(e.atCycleMatches) {
			e.atCycleIndex++
		}
		e.applyAtCycleTag(e.atCycleMatches[e.atCycleIndex])
		return
	}
	if e.atSelected+1 < len(e.atMatches) {
		e.atSelected++
	}
}

func (e *multilineEditor) clearAtStateAfterInsert() {
	e.atMatches = nil
	e.atSelected = 0
	e.atCtx = atmention.AtContext{}
}

func (e *multilineEditor) clearAtCycle() {
	e.atCycleActive = false
	e.atCycleStart = 0
	e.atCycleMatches = nil
	e.atCycleIndex = 0
}

func (e *multilineEditor) completeAtMentionTab() bool {
	if e.atCycleActive {
		if len(e.atCycleMatches) <= 1 {
			return false
		}
		e.atCycleIndex = (e.atCycleIndex + 1) % len(e.atCycleMatches)
		return e.applyAtCycleTag(e.atCycleMatches[e.atCycleIndex])
	}
	ctx := e.atCtx
	if !ctx.Active {
		ctx = atmention.AtContextAt(e.lines[e.row], e.col)
	}
	if !ctx.Active {
		return false
	}
	e.atCtx = ctx
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
	e.atCycleActive = true
	e.atCycleStart = ctx.ReplaceStart
	e.atCycleMatches = append([]atmention.Entry(nil), e.atMatches...)
	e.atCycleIndex = e.atSelected
	if e.atCycleIndex >= len(e.atCycleMatches) {
		e.atCycleIndex = 0
	}
	return e.applyAtCycleTag(e.atCycleMatches[e.atCycleIndex])
}

func (e *multilineEditor) completeAtMentionAccept() bool {
	if e.atCycleActive {
		e.insertRuneRaw(' ')
		e.clearAtCycle()
		e.clearAtStateAfterInsert()
		return true
	}
	ctx := atmention.AtContextAt(e.lines[e.row], e.col)
	if !ctx.Active {
		return false
	}
	e.atCtx = ctx
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

func (e *multilineEditor) applyAtCycleTag(sel atmention.Entry) bool {
	entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
	if err != nil || len(entries) == 0 {
		return false
	}
	tag := "@" + atmention.ShortTag(sel.RelPath, entries)
	line := e.lines[e.row]
	start := e.atCycleStart
	if start > len(line) {
		return false
	}
	end := e.col
	if end < start {
		end = start
	}
	e.lines[e.row] = append(append(line[:start], []rune(tag)...), line[end:]...)
	e.col = start + len(tag)
	e.atSelected = e.atCycleIndex
	e.atMatches = e.atCycleMatches
	return true
}

func (e *multilineEditor) insertAtTag(sel atmention.Entry, withSpace bool) bool {
	entries, err := replcomplete.AtIndexEntries(context.Background(), e.host.CompleteEnv)
	if err != nil || len(entries) == 0 {
		return false
	}
	tag := "@" + atmention.ShortTag(sel.RelPath, entries)
	line := e.lines[e.row]
	start := e.atCtx.ReplaceStart
	if start > e.col || start > len(line) {
		return false
	}
	var extra []rune
	if withSpace {
		extra = []rune{' '}
	}
	newLine := append(append(append(line[:start], []rune(tag)...), extra...), line[e.col:]...)
	e.lines[e.row] = newLine
	e.col = start + len(tag) + len(extra)
	e.clearAtCycle()
	e.clearAtStateAfterInsert()
	return true
}
