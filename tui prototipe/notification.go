package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) notification(raw string, index, width int) string {
	_, message, found := strings.Cut(raw, "\n")
	if !found {
		message = raw
	}
	cardWidth := max(8, min(44, width-6))
	inner := cardWidth - 4
	summary, detail, split := strings.Cut(message, "|")
	summary = strings.TrimSpace(summary)
	detail = strings.TrimSpace(detail)
	metadata := bold("System", ink) + "  " + text("10/25/2026, 10:30", muted)
	copyKey := strings.TrimSuffix(m.sectionKey(index, ""), ".")
	controls := clickZone("notification_copy:"+copyKey, text("⧉", muted)) + text("  ×", muted)
	content := []string{spacedPair(metadata, controls, inner)}
	if !split || summary == "" || detail == "" {
		content = append(content, lipgloss.NewStyle().Width(inner).Render(text(message, ink)))
	} else {
		key := m.sectionKey(index, "notification")
		arrow := "▸"
		if m.sections[key] {
			arrow = "▾"
		}
		label := lipgloss.NewStyle().Width(inner).Render(text(summary+" "+arrow, ink))
		content = append(content, clickZone("section:"+key, label))
		if m.sections[key] {
			content = append(content, lipgloss.NewStyle().Width(inner).Render(text(detail, muted)))
		}
	}
	lines := strings.Split(strings.Join(content, "\n"), "\n")
	rows := []string{text("╭"+strings.Repeat("─", cardWidth-2)+"╮", border)}
	for _, line := range lines {
		rows = append(rows, text("│", border)+" "+fit(line, inner, 1)+" "+text("│", border))
	}
	rows = append(rows, text("╰", border)+notificationProgress(cardWidth-2)+text("╯", border))
	padding := strings.Repeat(" ", max(0, width-cardWidth-3))
	for i := range rows {
		rows[i] = padding + rows[i]
	}
	return strings.Join(rows, "\n")
}
