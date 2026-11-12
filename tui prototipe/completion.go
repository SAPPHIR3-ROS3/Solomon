package main

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type slashCommand struct{ name, description string }

var slashCommands = []slashCommand{
	{"/agent", "Agent mode · tools and Go orchestration"},
	{"/artifacts", "Open the complete artifact gallery"},
	{"/chat", "Chat mode · web and documentation"},
	{"/clear", "Start a new thread"},
	{"/demo", "Replay the full archived editor conversation"},
	{"/help", "Show keyboard shortcuts"},
	{"/image", "Show the actual embedded PNG image"},
	{"/new", "Start a new thread"},
	{"/components", "Open the live component map"},
}

func (m model) slashMatches() []slashCommand {
	draft := string(m.draft)
	if m.guiMenu != "" || m.palette || m.menuHidden || !strings.HasPrefix(draft, "/") || strings.ContainsAny(draft, " \n\t") {
		return nil
	}
	result := []slashCommand{}
	for _, command := range slashCommands {
		if strings.HasPrefix(command.name, draft) {
			result = append(result, command)
		}
	}
	return result
}
func (m model) slashKey(key tea.KeyMsg) (model, bool) {
	matches := m.slashMatches()
	if len(matches) > 0 {
		switch m.liveCfg.Action("suggestions", key.String()) {
		case "previous":
			m.suggestion = (m.suggestion + len(matches) - 1) % len(matches)
			return m, true
		case "next":
			m.suggestion = (m.suggestion + 1) % len(matches)
			return m, true
		case "complete":
			m.draft = []rune(matches[m.suggestion%len(matches)].name)
			m.suggestion = 0
			return m, true
		case "accept":
			m.draft = []rune(matches[m.suggestion%len(matches)].name)
			m.suggestion = 0
			return m, false
		case "dismiss":
			m.menuHidden = true
			return m, true
		}
	}
	if key.Type == tea.KeyRunes || key.String() == "backspace" || key.String() == "ctrl+u" {
		m.suggestion = 0
		m.menuHidden = false
	}
	return m, false
}
func (m model) slashMenu(width int) string {
	matches := m.slashMatches()
	if len(matches) == 0 {
		return ""
	}
	selected := m.suggestion % len(matches)
	start := max(0, selected-activeInterface.Layout.SuggestionRows+1)
	end := min(len(matches), start+activeInterface.Layout.SuggestionRows)
	rows := []string{}
	for i := start; i < end; i++ {
		marker := "  "
		color := muted
		if i == selected {
			marker = "› "
			color = gold
		}
		row := text(marker+matches[i].name, color) + text("  "+matches[i].description, muted)
		if mockupCommand(matches[i].name) {
			row = mockupSurface(row, max(1, width-4), 1)
		}
		rows = append(rows, clickZone("slash:"+strconv.Itoa(i), row))
	}
	rows = append(rows, text("↑↓ select · "+keyCaption(configuredHint("suggestions", "complete"))+" complete · "+keyCaption(configuredHint("suggestions", "accept"))+" run · "+keyCaption(configuredHint("suggestions", "dismiss"))+" dismiss", muted))
	return box(joinRows(rows), width, len(rows)+2, gold)
}
func joinRows(rows []string) string { return strings.Join(rows, "\n") }
func (m model) composerDock(width int) string {
	if m.guiMenu != "" {
		return m.guiPicker(width) + "\n" + m.composer(width)
	}
	menu := m.slashMenu(width)
	if menu == "" {
		return m.composer(width)
	}
	return menu + "\n" + m.composer(width)
}
