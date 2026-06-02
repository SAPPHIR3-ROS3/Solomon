package repl

type InputHistoryTest struct {
	*inputHistory
}

func NewInputHistoryForTest() *InputHistoryTest {
	return &InputHistoryTest{inputHistory: newInputHistory()}
}

func (h *InputHistoryTest) Add(s string) {
	h.add(s)
}

func (h *InputHistoryTest) Prev(draft string) (string, bool) {
	return h.prev(draft)
}

func (h *InputHistoryTest) Next() (string, bool) {
	return h.next()
}

type MultilineEditorTest struct {
	*multilineEditor
}

func NewMultilineEditorForTest(loop *Loop, history *InputHistoryTest, lines []string, row, col, width int) *MultilineEditorTest {
	rlines := make([][]rune, len(lines))
	for i, s := range lines {
		rlines[i] = []rune(s)
	}
	var hist *inputHistory
	if history != nil {
		hist = history.inputHistory
	} else {
		hist = newInputHistory()
	}
	if loop == nil {
		loop = &Loop{}
	}
	return &MultilineEditorTest{multilineEditor: &multilineEditor{
		loop:    loop,
		history: hist,
		lines:   rlines,
		row:     row,
		col:     col,
		width:   width,
	}}
}

func (e *MultilineEditorTest) Up() {
	e.up()
}

func (e *MultilineEditorTest) String() string {
	return e.string()
}

func (e *MultilineEditorTest) Row() int {
	return e.row
}

func (e *MultilineEditorTest) Col() int {
	return e.col
}

func (e *MultilineEditorTest) Complete() bool {
	return e.complete()
}

func (e *MultilineEditorTest) Line(i int) string {
	return string(e.lines[i])
}

func CommonRunePrefixForTest(candidates [][]rune) []rune {
	return commonRunePrefix(candidates)
}
