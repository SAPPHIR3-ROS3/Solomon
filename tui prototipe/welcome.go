package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func bannerPanel(width int) string {
	logo := strings.Split(banner(), "\n")
	inner := width - 2
	logoWidth := lipgloss.Width(banner()) + 2
	split := inner >= logoWidth+24
	left := logoWidth
	right := inner - left - 1
	paint := func(s string) string { return bold(s, gold) }
	welcome := text("Welcome to ", ink) + bold("Solomon", gold)
	heading := "─" + welcome
	rows := []string{}
	if split {
		rows = append(rows, paint("┌─")+welcome+paint(strings.Repeat("─", max(0, left-1-lipgloss.Width(welcome)))+"┬"+strings.Repeat("─", right)+"┐"))
	} else {
		rows = append(rows, paint("┌")+heading+paint(strings.Repeat("─", max(0, inner-lipgloss.Width(heading)))+"┐"))
	}
	info := []string{bold("Solomon", gold), text("new thread", muted), "", text("/resume  Recent chats", ink), mockupTag("/demo    Archived chat"), mockupTag("/artifacts  UI gallery"), text("/agent   Code mode", green), text("/chat    Web and docs", green), text("/help    Shortcuts", ink)}
	for i, row := range logo {
		body := fit(row, inner, 1)
		if split {
			r := ""
			if i < len(info) {
				r = info[i]
			}
			body = fit(row, left, 1) + paint("│") + fit(" "+r, right, 1)
		}
		rows = append(rows, paint("│")+body+paint("│"))
	}
	divider := strings.Repeat("─", inner)
	if split {
		divider = strings.Repeat("─", left) + "┴" + strings.Repeat("─", right)
	}
	rows = append(rows, paint("├"+divider+"┤"))
	rows = append(rows, paint("│")+fit(text(" new thread · no model connected", green), inner, 1)+paint("│"))
	rows = append(rows, paint("│")+fit(text(" ~/Solomon (main)", ink), inner, 1)+paint("│"))
	rows = append(rows, paint("└"+strings.Repeat("─", inner)+"┘"))
	return strings.Join(rows, "\n")
}
func (m model) welcome(width, height int) string {
	if m.variant == 4 {
		return m.guiWelcome(width, height)
	}
	if len(m.slashMatches()) > 0 {
		dock := m.composerDock(width)
		return fit(bannerPanel(width), width, max(1, height-lipgloss.Height(dock)-1)) + "\n" + dock + "\n" + m.metadata(width)
	}
	groupWidth := width
	if activeInterface.Layout.HomeWidth != 96 {
		groupWidth = min(groupWidth, activeInterface.Layout.HomeWidth)
	}
	if m.variant == 2 {
		groupWidth = min(groupWidth, 100)
	}
	if m.variant == 3 {
		groupWidth = min(groupWidth, 90)
	}
	if m.variant == 4 {
		groupWidth = min(groupWidth, 104)
	}
	group := strings.Join([]string{bannerPanel(groupWidth), m.composer(groupWidth), m.metadata(groupWidth), mockupSurface("/demo archived thread  ·  /artifacts gallery", groupWidth, 1)}, "\n")
	horizontal, vertical := lipgloss.Left, lipgloss.Top
	if m.variant == 2 || m.variant == 4 {
		horizontal, vertical = lipgloss.Center, lipgloss.Center
	}
	return lipgloss.Place(width, height, horizontal, vertical, group)
}
