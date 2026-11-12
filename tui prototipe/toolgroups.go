package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type toolActivityGroup struct {
	first, last, count int
	running            bool
}

func toolActivityGroups(messages []message, enabled bool) map[int]toolActivityGroup {
	groups := map[int]toolActivityGroup{}
	if !enabled {
		return groups
	}
	for start := 0; start < len(messages); {
		end := start + 1
		for end < len(messages) && messages[end].role != "YOU" {
			end++
		}
		group := toolActivityGroup{first: -1, last: -1}
		for i := start; i < end; i++ {
			if messages[i].role != "TOOLCALL" {
				continue
			}
			if group.first < 0 {
				group.first = i
			}
			group.last = i
			group.count++
			var tool recordedTool
			if json.Unmarshal([]byte(messages[i].text), &tool) == nil && tool.Status == "running" {
				group.running = true
			}
		}
		if group.count > 0 {
			for i := group.first; i <= group.last; i++ {
				groups[i] = group
			}
		}
		start = end
	}
	return groups
}

func (m model) toolActivityControl(group toolActivityGroup, closed bool, width int) []string {
	label := "Collapse tool calls"
	if closed {
		label = fmt.Sprintf("Show %d tool calls", group.count)
	}
	cardWidth := max(8, width)
	content := lipgloss.NewStyle().Width(cardWidth - 4).Align(lipgloss.Center).Render(text(label, muted))
	button := clickZone("section:"+m.sectionKey(group.first, "toolgroup"), box(content, cardWidth, 3, border, true))
	return strings.Split(button, "\n")
}
