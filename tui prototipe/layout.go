package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var names = []string{"", "ATELIER", "ORBIT", "PAPER", "MISSION"}

func (m model) View() string {
	return m.zones.Capture(animateNotification(animateWorkedFor(animateModeProgress(animateToolIndicators(m.renderView())))), lipgloss.Width)
}
func (m model) renderView() string {
	if m.width < 42 || m.height < 24 {
		return "Solomon\nResize to at least 42 × 24.\nCtrl+C to exit."
	}
	if m.componentOpen {
		return m.componentMap()
	}
	if m.palette {
		return fit(m.commandPalette(), m.width, m.height)
	}

	width := m.contentWidth()
	var page string
	if len(m.messages) == 0 && len(m.streaming) == 0 {
		page = m.welcome(width, m.height-1)
	} else {
		dock := m.composerDock(width)
		bodyHeight := m.height - lipgloss.Height(dock) - 2
		page = strings.Join([]string{m.conversation(width, bodyHeight), dock, m.metadata(width)}, "\n")
	}
	return fit(lipgloss.NewStyle().MarginLeft(activeInterface.Layout.Margin).Render(page)+"\n"+m.footer(), m.width, m.height)
}

func (m model) footer() string {
	if m.liveError != "" {
		return mockupSurface(" Configuration rejected · "+m.liveError, m.width, 1)
	}
	if m.devStatus != "" {
		return mockupSurface(" Build failed · "+m.devStatus, m.width, 1)
	}
	label := ""
	for position, i := range []int{2, 4} {
		if position > 0 {
			label += "  "
		} else {
			label = " "
		}
		label += clickZone(fmt.Sprintf("layout:%d", i), strings.ToUpper(configuredHint("global", fmt.Sprintf("layout_%d", i)))+" "+strings.Title(strings.ToLower(names[i])))
	}
	if m.width >= 100 {
		label += "  │ " + clickZone("action:components", keyCaption(configuredHint("global", "components"))+" components") + "  " + clickZone("action:quit", configuredHint("global", "quit")) + " exit"
	}
	return mockupSurface(text(label, ink), m.width, 1)
}
func (m model) conversation(width, height int) string {
	title := bold("Solomon", gold) + text("  ~/Solomon  (main)", muted)
	rows, _ := m.transcriptLines(width)
	available := max(1, height-2)
	top := max(0, len(rows)-available-min(m.scroll, max(0, len(rows)-available)))
	end := min(len(rows), top+available)
	if len(m.streaming) > 0 && width >= 80 {
		activity := text("Working", gold)
		title += strings.Repeat(" ", max(1, width-lipgloss.Width(title)-lipgloss.Width(activity))) + activity
	}
	title += scrollDiagnostic(top, end, len(rows))
	visible := append([]string{}, rows[top:end]...)
	if end < len(rows) && len(visible) >= 3 {
		m.overlayBottomButton(visible, "action:scroll_bottom", width)
	}
	return fit(title, width, 1) + "\n" + line(width) + "\n" + fit(strings.Join(visible, "\n"), width, available)
}
func (m model) commandPalette() string {
	width := min(68, m.width-4)
	content := bold("COMMAND CENTER", gold) + "\n\n"
	for _, i := range []int{2, 4} {
		prefix := "  "
		if i == m.variant {
			prefix = "› "
		}
		content += clickZone(fmt.Sprintf("layout:%d", i), text(fmt.Sprintf("%s%s  %s", prefix, strings.ToUpper(configuredHint("global", fmt.Sprintf("layout_%d", i))), names[i]), ink)) + "\n"
	}
	content += "\n" + clickZone("action:new_thread", text("Ctrl+N  New thread", gold)) + "\n" + clickZone("action:toggle_details", text("Ctrl+D  Tool details", muted)) + "\n\n" + text("Enter  next layout     Esc  close", muted)
	dialog := mockupSurface(box(content, width, 16, border), width, 16)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}
func snapshot(m model) string { return strings.TrimRight(m.View(), "\n") }
