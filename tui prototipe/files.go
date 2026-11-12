package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func fileCounts(added, removed int) string {
	return text(fmt.Sprintf("+%d", added), "#32d49a") + "   " + text(fmt.Sprintf("-%d", removed), "#ff626b")
}

func (m model) fileSummary(width int) string {
	width = max(12, width-6)
	inner := width - 4
	row := func(label string, added, removed int) string {
		counts := fileCounts(added, removed)
		available := max(1, inner-lipgloss.Width(counts)-2)
		return spacedPair(text(lipgloss.NewStyle().MaxWidth(available).Render(label), ink), counts, inner)
	}
	rows := []string{clickZone("files:header", row("3 changed files", 42, 8))}
	if m.componentState != 0 {
		for _, folder := range []struct {
			name           string
			added, removed int
			files          []string
			adds, removes  []int
		}{
			{"gui/src/editor", 38, 8, []string{"EditorPage.tsx", "ImagePreview.tsx"}, []int{24, 14}, []int{6, 2}},
			{"test", 4, 0, []string{"image_test.go"}, []int{4}, []int{0}},
		} {
			key := fmt.Sprintf("component-%d.files.%s", m.componentIndex, strings.ReplaceAll(folder.name, "/", "."))
			open := (m.componentState == 2) != m.sections[key]
			arrow := "▸ "
			if open {
				arrow = "▾ "
			}
			rows = append(rows, clickZone("section:"+key, row(arrow+folder.name, folder.added, folder.removed)))
			if open {
				for i, name := range folder.files {
					rows = append(rows, row("    "+name, folder.adds[i], folder.removes[i]))
				}
			}
		}
	}
	content := strings.Join(rows, "\n")
	return box(content, width, lipgloss.Height(content)+2, border, true)
}
