package main

import "github.com/charmbracelet/lipgloss"

type scrollView struct {
	rows               []string
	starts             []int
	height, top, limit int
}

func (m model) scrollView() scrollView {
	width := m.contentWidth()
	height := max(1, m.height-lipgloss.Height(m.composerDock(width))-4)
	rows, starts := m.transcriptLines(width)
	limit := max(0, len(rows)-height)
	return scrollView{rows: rows, starts: starts, height: height, top: limit - min(max(0, m.scroll), limit), limit: limit}
}
func (m model) scrollToTop() model {
	if len(m.messages) == 0 && len(m.streaming) == 0 {
		m.scroll = 0
		return m
	}
	m.scroll = m.scrollView().limit
	return m
}
func (m model) moveScroll(delta int) model {
	if m.palette || len(m.messages) == 0 && len(m.streaming) == 0 {
		return m
	}
	limit := m.scrollView().limit
	m.scroll = min(limit, max(0, min(limit, max(0, m.scroll))+delta))
	return m
}
func (m model) scrollAnchor() (int, int) {
	view := m.scrollView()
	index := 0
	for i, start := range view.starts {
		if start > view.top {
			break
		}
		index = i
	}
	if len(view.starts) == 0 {
		return 0, 0
	}
	return index, view.top - view.starts[index]
}
func (m model) restoreAnchor(index, offset int) model {
	view := m.scrollView()
	if len(view.starts) == 0 {
		m.scroll = 0
		return m
	}
	index = min(max(0, index), len(view.starts)-1)
	end := len(view.rows)
	if index+1 < len(view.starts) {
		end = view.starts[index+1]
	}
	top := view.starts[index] + min(max(0, offset), max(0, end-view.starts[index]-1))
	m.scroll = max(0, view.limit-min(view.limit, top))
	return m
}

func (m model) toggleArtifact() model {
	if len(m.messages) == 0 {
		return m
	}
	index, offset := m.scrollAnchor()
	if m.folded == nil {
		m.folded = map[int]bool{}
	}
	m.folded[index] = !m.folded[index]
	m.foldVersion++
	return m.restoreAnchor(index, offset)
}
