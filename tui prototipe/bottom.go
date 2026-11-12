package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m model) composerBorderColor() string {
	if m.variant == 4 {
		return activeInterface.Colors["accent"]
	}
	return cyan
}

func (m model) overlayBottomButton(rows []string, id string, width int) {
	if len(rows) < 3 {
		return
	}
	left := max(0, (width-5)/2)
	for i, border := range []string{"╭───╮", "│ ↓ │", "╰───╯"} {
		index := len(rows) - 3 + i
		row := rows[index]
		prefix := ansi.Cut(row, 0, left)
		prefix += strings.Repeat(" ", max(0, left-ansi.StringWidth(prefix)))
		button := clickZone(id, text(border, m.composerBorderColor()))
		rows[index] = prefix + button + ansi.Cut(row, left+5, width)
	}
}
