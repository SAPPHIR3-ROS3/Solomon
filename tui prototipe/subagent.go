package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) subagentCard(raw string, width int) string {
	_, body, found := strings.Cut(raw, "\n")
	if !found {
		body = raw
	}
	cards := []string{}
	name, task, status := "", "", "queued"
	modelName, reasoning, fast := "", "", false
	flush := func() {
		if name == "" {
			return
		}
		intent := task
		if intent == "" {
			intent = name
		}
		dot := toolStatusIndicator(status)
		if status == "queued" {
			dot = text("○", muted)
		}
		heading := bold("Subagent", ink)
		if modelName != "" {
			heading += "  " + text("["+modelName+"]", muted)
		}
		if reasoning != "" || fast {
			heading += " " + text("["+reasoning, muted)
			if fast {
				if reasoning != "" {
					heading += " "
				}
				heading += text("ϟ", gold)
			}
			heading += text("]", muted)
		}
		if status == "queued" {
			heading += "  " + text("Queued", muted)
		}
		innerWidth := max(1, width-10)
		content := lipgloss.NewStyle().Width(innerWidth).Render(heading) + "\n" + text(lipgloss.NewStyle().Width(innerWidth).Render(intent), ink)
		framed := box(content, max(8, width-6), lipgloss.Height(content)+2, border, true)
		rows := strings.Split(framed, "\n")
		for i := range rows {
			gutter := "   "
			if i == 1 {
				gutter = " " + dot + " "
			}
			rows[i] = gutter + rows[i]
		}
		cards = append(cards, strings.Join(rows, "\n"))
	}
	for _, line := range strings.Split(body, "\n") {
		value := strings.TrimSpace(line)
		if strings.HasPrefix(value, "● ") || strings.HasPrefix(value, "○ ") {
			flush()
			fields := strings.Fields(value)
			name, task, status = "Subagent", "", "queued"
			modelName, reasoning, fast = "", "", false
			if len(fields) > 1 {
				name = fields[1]
			}
			if strings.HasSuffix(value, "completed") {
				status = "success"
			}
			continue
		}
		label, content, ok := strings.Cut(value, ":")
		if ok {
			content = strings.TrimSpace(content)
			switch label {
			case "Task":
				task = content
			case "Model":
				modelName = content
			case "Reasoning":
				reasoning = content
			case "Fast":
				fast = content == "true"
			}
		}
	}
	flush()
	if len(cards) == 0 {
		return m.artifact("SUBAGENT", "Subagent", body, width)
	}
	return strings.Join(cards, "\n\n")
}
