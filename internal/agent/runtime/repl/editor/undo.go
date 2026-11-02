package editor

const undoLimit = 100
const undoByteLimit = 1024 * 1024

type editSnapshot struct {
	text string
	row  int
	col  int
}

func (e *multilineEditor) snapshot() editSnapshot {
	return editSnapshot{text: e.string(), row: e.row, col: e.col}
}

func appendSnapshot(stack []editSnapshot, state editSnapshot) []editSnapshot {
	stack = append(stack, state)
	size := 0
	start := len(stack)
	for start > 0 && len(stack)-start < undoLimit {
		next := len(stack[start-1].text)
		if size+next > undoByteLimit {
			break
		}
		size += next
		start--
	}
	return append([]editSnapshot(nil), stack[start:]...)
}

func (e *multilineEditor) handle(key editorKey) (bool, string, error) {
	undo := key.seq == "" && key.r == 26
	redo := key.seq == "" && key.r == 25
	switch key.seq {
	case "\x1b[122;5u", "\x1b[27;5;122~":
		undo = true
	case "\x1b[122;6u", "\x1b[90;6u", "\x1b[27;6;90~", "\x1b[27;6;122~":
		redo = true
	}
	if undo || redo {
		e.restoreEdit(redo)
		e.recomputeSuggest()
		e.recomputeAtPicker()
		e.refresh()
		return false, "", nil
	}
	before := e.snapshot()
	epoch := e.historyEpoch
	done, line, err := e.handleKey(key)
	if epoch == e.historyEpoch && before.text != e.string() {
		e.undoStack = appendSnapshot(e.undoStack, before)
		e.redoStack = nil
	}
	return done, line, err
}

func (e *multilineEditor) restoreEdit(redo bool) {
	source, target := &e.undoStack, &e.redoStack
	if redo {
		source, target = target, source
	}
	if len(*source) == 0 {
		return
	}
	state := (*source)[len(*source)-1]
	*target = appendSnapshot(*target, e.snapshot())
	*source = (*source)[:len(*source)-1]
	e.setString(state.text, state.row)
	e.col = state.col
	e.clearSuggest()
	e.history.resetNav()
}
