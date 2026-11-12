package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	border = "#174875"
	ink    = "#e8e5df"
	muted  = "#808080"
	gold   = "#ffc704"
	cyan   = "#00FFFF"
	green  = "#00FF00"
	violet = "#AA55FF"
)

func text(s, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}
func bold(s, color string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(s)
}
func line(width int) string { return text(strings.Repeat("─", max(0, width)), border) }
func fit(body string, width, height int) string {
	return lipgloss.NewStyle().Width(max(1, width)).Height(max(1, height)).MaxWidth(max(1, width)).MaxHeight(max(1, height)).Render(body)
}
func box(body string, width, height int, color string, heavy ...bool) string {
	inner := max(1, width-4)
	content := strings.Split(body, "\n")
	tl, tr, bl, br, horizontal, vertical := interfaceBorder()
	if len(heavy) > 0 && heavy[0] {
		tl, tr, bl, br, horizontal, vertical = "╭", "╮", "╰", "╯", "─", "│"
	}
	rows := []string{bold(tl+strings.Repeat(horizontal, max(1, width-2))+tr, color)}
	for i := 0; i < max(1, height-2); i++ {
		row := ""
		if i < len(content) {
			row = content[i]
		}
		rows = append(rows, bold(vertical, color)+" "+fit(row, inner, 1)+" "+bold(vertical, color))
	}
	rows = append(rows, bold(bl+strings.Repeat(horizontal, max(1, width-2))+br, color))
	return strings.Join(rows, "\n")
}
func (m model) accent() string { return gold }
func (m model) composer(width int) string {
	if m.variant == 4 {
		return m.guiComposer(width)
	}
	value := string(m.draft)
	if value == "" {
		value = text("▏", cyan) + text(configuredText("placeholder"), muted)
	} else {
		value = text(value, ink) + text("▏", cyan)
	}
	wrapped := lipgloss.NewStyle().Width(max(1, width-10)).Render(value)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > activeInterface.Layout.MaxInputRows {
		lines = lines[len(lines)-activeInterface.Layout.MaxInputRows:]
	}
	input := bold("You: ", cyan) + strings.Join(lines, "\n")
	return box(input, width, len(lines)+2, cyan)
}
func (m model) metadata(width int) string {
	hint := text(inputHint(), muted)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(hint)
}
