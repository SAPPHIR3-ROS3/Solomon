package editor

import "bytes"

type HistoryTest struct {
	*History
}

func NewHistoryForTest() *HistoryTest {
	return &HistoryTest{History: NewHistory()}
}

func (h *HistoryTest) Add(s string) {
	h.History.Add(s, false)
}

func (h *HistoryTest) AddWithMode(s string, shellFirst bool) {
	h.History.Add(s, shellFirst)
}

func (h *HistoryTest) ShellMatch(prefix string) string {
	return h.shellMatch(prefix)
}

func (h *HistoryTest) Prev(draft string) (string, bool) {
	return h.prev(draft)
}

func (h *HistoryTest) Next() (string, bool) {
	return h.next()
}

type MultilineEditorTest struct {
	*multilineEditor
}

func NewMultilineEditorForTest(host Host, history *HistoryTest, lines []string, row, col, width int) *MultilineEditorTest {
	rlines := make([][]rune, len(lines))
	for i, s := range lines {
		rlines[i] = []rune(s)
	}
	var hist *History
	if history != nil {
		hist = history.History
	} else {
		hist = NewHistory()
	}
	return &MultilineEditorTest{multilineEditor: &multilineEditor{
		host:    host,
		history: hist,
		lines:   rlines,
		row:     row,
		col:     col,
		width:   width,
	}}
}

func (e *MultilineEditorTest) SetHeight(height int) {
	e.height = height
}

func (e *MultilineEditorTest) TotalVisualRows() int {
	return e.totalContentVisualRows()
}

func (e *MultilineEditorTest) ViewportTop() int {
	return e.viewportTop
}

func (e *MultilineEditorTest) RenderedRows() int {
	return e.rendered
}

func (e *MultilineEditorTest) CursorLine() int {
	return e.cursorLine
}

func (e *MultilineEditorTest) RefreshOutput() string {
	var out bytes.Buffer
	e.out = &out
	e.refresh()
	return out.String()
}

func (e *MultilineEditorTest) FinishOutput() string {
	var out bytes.Buffer
	e.out = &out
	e.finish()
	return out.String()
}

func (e *MultilineEditorTest) InsertPaste(s string) {
	e.insertPaste(s)
}

func (e *MultilineEditorTest) InsertString(s string) {
	e.insertString(s)
}

func (e *MultilineEditorTest) TypeString(s string) {
	for _, r := range s {
		e.insertRune(r)
	}
}

func (e *MultilineEditorTest) Backspace() {
	e.backspace()
}

func (e *MultilineEditorTest) Up() {
	e.up()
}

func (e *MultilineEditorTest) Down() {
	e.down()
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

func (e *MultilineEditorTest) RecomputeSuggest() {
	e.recomputeSuggest()
}

func (e *MultilineEditorTest) SuggestSuffix() string {
	return string(e.suggestSuffix)
}

func (e *MultilineEditorTest) AcceptSuggestAll() {
	e.acceptSuggest(false)
}

func CommonRunePrefixForTest(candidates [][]rune) []rune {
	return commonRunePrefix(candidates)
}

func ShellPrefixNormalizedForTest(buffer string, shellFirst bool) string {
	return shellPrefixNormalized(buffer, shellFirst)
}

func VisualRowsWithGhostForTest(width int, prompt, line, ghost string) int {
	e := &multilineEditor{width: width}
	if ghost == "" {
		return e.visualRows(prompt, []rune(line))
	}
	combined := append(append([]rune(nil), []rune(line)...), []rune(ghost)...)
	return e.visualRows(prompt, combined)
}
