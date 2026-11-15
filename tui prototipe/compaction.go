package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) compactionCard(width int) string {
	width = max(8, width)
	prefix := fmt.Sprintf("component-%d.0.compaction.", m.componentIndex)
	background := lipgloss.Color("#a18d43")
	header := func(title, key string, open bool) string {
		arrow := "▾"
		if open {
			arrow = "▴"
		}
		label := lipgloss.NewStyle().Width(width-2).Align(lipgloss.Center).Bold(true).Foreground(lipgloss.Color("#a83b3b")).Background(background).Render(title)
		indicator := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(background).Render(arrow + " ")
		row := label + indicator
		return clickZone("section:"+prefix+key, row)
	}
	body := func(value string) string {
		rendered := renderMarkdown(value, max(1, width-4))
		style := lipgloss.NewStyle().Width(width).Padding(0, 2).Background(background).Foreground(lipgloss.Color("#ffffff"))
		return style.Render(rendered)
	}
	open := m.sections[prefix+"card"]
	rows := []string{header("Context compacted", "card", open)}
	if !open {
		return strings.Join(rows, "\n")
	}
	for _, item := range []struct{ key, title, content string }{
		{"summary", "Summary", "## Current task\nAdd read-only image previews to the editor.\n\n- Preserve binary files\n- Keep text autosave unchanged\n- Verify PNG and JPEG previews"},
		{"recent", "Recent messages", "You: Verify image tabs do not trigger autosave.\n\nAssistant: Image previews use the binary endpoint and remain read-only."},
	} {
		expanded := !m.sections[prefix+item.key]
		rows = append(rows, header(item.title, item.key, expanded))
		if expanded {
			rows = append(rows, body(item.content))
		}
	}
	return strings.Join(rows, "\n")
}
