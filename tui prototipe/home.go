package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var previewModels = []string{"gpt-6.1-sol", "Claude Sonnet", "Codex", "Local model"}
var previewReasoning = []string{"None", "Low", "Medium", "High"}

func spacedPair(left, right string, width int) string {
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-lipgloss.Width(right))) + right
}
func (m model) guiComposer(width int) string {
	inner := max(1, width-4)
	value := string(m.draft)
	if value == "" {
		value = text("▏", gold) + text(configuredText("gui_placeholder"), muted)
	} else {
		value = text(value, ink) + text("▏", gold)
	}
	rows := strings.Split(lipgloss.NewStyle().Width(inner).Render(value), "\n")
	if len(rows) > activeInterface.Layout.MaxInputRows {
		rows = rows[len(rows)-activeInterface.Layout.MaxInputRows:]
	}
	for len(rows) < activeInterface.Layout.ComposerRows {
		rows = append(rows, "")
	}
	mode := bold("♛ Agent", gold)
	if m.selected == 1 {
		mode = text("◇ Chat", cyan)
	}
	modelName := text(previewModels[m.modelChoice%len(previewModels)]+" ▾", ink)
	reasoning := text(previewReasoning[m.reasoningChoice%len(previewReasoning)]+" ▾", muted)
	send := bold("↑", gold)
	if len(m.streaming) > 0 {
		send = bold("■", gold)
	}
	toolbar := clickZone("toolbar:model", modelName) + text("  │  ", border) + clickZone("toolbar:reasoning", reasoning) + "  " + clickZone("toolbar:mode", mode)
	if inner < 56 {
		toolbar = clickZone("toolbar:model", text("Model ▾", ink)) + "  " + clickZone("toolbar:reasoning", reasoning) + "  " + clickZone("toolbar:mode", mode)
	}
	rows = append(rows, spacedPair(toolbar, clickZone("toolbar:send", send), inner))
	return box(strings.Join(rows, "\n"), width, len(rows)+2, activeInterface.Colors["accent"])
}
func (m model) guiLockup(width, height int) string {
	raw := strings.Split(banner(), "\n")
	logoWidth := lipgloss.Width(banner())
	rows := make([]string, len(raw))
	left := max(0, (width-logoWidth)/2)
	for i, row := range raw {
		rows[i] = fit(strings.Repeat(" ", left)+row, width, 1)
	}
	name := os.Getenv("SOLOMON_PREVIEW_NAME")
	if name == "" {
		name = "User"
	}
	greeting := text(configuredText("greeting"), ink) + bold(name, ink)
	if lipgloss.Width(greeting) > width {
		greeting = bold("Welcome back", ink)
	}
	middle := len(rows) / 2
	start := max(0, (width-lipgloss.Width(greeting))/2)
	row := rows[middle]
	rows[middle] = ansi.Cut(row, 0, start) + greeting + ansi.Cut(row, start+lipgloss.Width(greeting), width)
	return strings.Join(rows, "\n")
}
func (m model) guiWelcome(width, height int) string {
	stageWidth := min(activeInterface.Layout.HomeWidth, max(1, width-4))
	if width < 60 {
		stageWidth = width
	}
	dock := m.composerDock(stageWidth)
	workspace := text(configuredText("workspace"), ink) + text(" ▾", muted)
	controls := text("main ▾", muted) + text("    worktree ▾", muted)
	notifications := text(configuredText("notifications"), muted)
	controls = spacedPair(controls, notifications, stageWidth)
	stats := text("new thread", gold) + "   " + mockupTag("1 archived chat") + "   " + text("~", muted) + text("0", cyan) + text(" + ", muted) + text("0", violet) + text(" + ", muted) + text("0", green) + text(" = 0 tokens", muted)
	if stageWidth < 58 {
		stats = text("new thread · ", muted) + mockupTag("1 archive")
	}
	lockup := m.guiLockup(stageWidth, lipgloss.Height(banner()))
	group := strings.Join([]string{lockup, fit(workspace, stageWidth, 1), dock, controls, fitCentered(stats, stageWidth)}, "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, group)
}
func fitCentered(value string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(value)
}
func (m model) guiPicker(width int) string {
	choices := previewModels
	title := configuredText("model_picker")
	selected := m.modelChoice
	if m.guiMenu == "reasoning" {
		choices = previewReasoning
		title = configuredText("reasoning_picker")
		selected = m.reasoningChoice
	}
	rows := []string{bold(title, gold)}
	for i, choice := range choices {
		marker := "  "
		color := muted
		if i == selected {
			marker = "› "
			color = ink
		}
		rows = append(rows, clickZone("picker:"+strconv.Itoa(i), text(marker+choice, color)))
	}
	rows = append(rows, text("↑↓ choose · "+keyCaption(configuredHint("picker", "accept"))+" confirm · "+keyCaption(configuredHint("picker", "dismiss"))+" close", muted))
	return box(strings.Join(rows, "\n"), width, len(rows)+2, gold)
}

func (m model) guiControlKey(key tea.KeyMsg) (model, bool) {
	switch m.liveCfg.Action("global", key.String()) {
	case "models":
		if m.guiMenu == "model" {
			m.guiMenu = ""
		} else {
			m.guiMenu = "model"
		}
		m.menuHidden = true
		return m, true
	case "reasoning":
		if m.guiMenu == "reasoning" {
			m.guiMenu = ""
		} else {
			m.guiMenu = "reasoning"
		}
		m.menuHidden = true
		return m, true
	}
	if m.guiMenu == "" {
		return m, false
	}
	selected := &m.modelChoice
	count := len(previewModels)
	if m.guiMenu == "reasoning" {
		selected = &m.reasoningChoice
		count = len(previewReasoning)
	}
	switch m.liveCfg.Action("picker", key.String()) {
	case "previous":
		*selected = (*selected + count - 1) % count
		return m, true
	case "next":
		*selected = (*selected + 1) % count
		return m, true
	case "accept", "dismiss":
		m.guiMenu = ""
		return m, true
	}
	return m, false
}
