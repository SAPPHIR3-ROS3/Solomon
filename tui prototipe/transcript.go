package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type transcriptKey struct {
	width, variant, count, revealed, streamLength, foldVersion int
	details                                                    bool
	first, last                                                *message
}
type transcriptCache struct {
	entries []entryRender
	key     transcriptKey
	valid   bool
	rows    []string
	starts  []int
}

func (m model) transcriptLines(width int) ([]string, []int) {
	key := transcriptKey{width: width, variant: m.variant, count: len(m.messages), revealed: m.revealed, streamLength: len(m.streaming), details: m.details, foldVersion: m.foldVersion}
	if len(m.messages) > 0 {
		key.first = &m.messages[0]
		key.last = &m.messages[len(m.messages)-1]
	}
	if m.cache != nil && m.cache.valid && m.cache.key == key {
		return m.cache.rows, m.cache.starts
	}
	previous := []entryRender{}
	if m.cache != nil {
		previous = m.cache.entries
	}
	messages := append([]message{}, m.messages...)
	if len(m.streaming) > 0 {
		messages = append(messages, message{"SOLOMON", string(m.streaming[:m.revealed]) + " ▍"})
	}
	entries := make([]entryRender, len(messages))
	rows := []string{}
	starts := []int{}
	groupsEnabled := !strings.HasPrefix(m.sectionPrefix, "component-") || componentNames[m.componentIndex%len(componentNames)] == "Transcript"
	groups := toolActivityGroups(messages, groupsEnabled)
	for index, entry := range messages {
		group, inGroup := groups[index]
		groupClosed := inGroup && !group.running && m.sections[m.sectionKey(group.first, "toolgroup")]
		if groupClosed {
			starts = append(starts, len(rows))
			if index == group.last {
				rows = append(rows, m.toolActivityControl(group, true, width)...)
			}
			continue
		}
		details := m.details
		if m.folded[index] {
			details = !details
		}
		itemKey := entryRenderKey{width: width, variant: m.variant, details: details, revision: interfaceRevision, sourceClosed: m.sections[m.sectionKey(index, "Source")], notificationOpen: m.sections[m.sectionKey(index, "notification")], resultClosed: m.sections[m.sectionKey(index, "Result")], queryClosed: m.sections[m.sectionKey(index, "Query")]}
		if index < len(m.messages) {
			itemKey.message = &m.messages[index]
		} else {
			itemKey.streaming = entry.text
		}
		starts = append(starts, len(rows))
		if index < len(previous) && previous[index].key == itemKey {
			entries[index] = previous[index]
		} else {
			entries[index] = entryRender{key: itemKey, rows: m.renderEntry(entry, index, width)}
		}
		entryRows := append([]string{}, entries[index].rows...)
		if inGroup && group.count > 1 {
			for line := range entryRows {
				connect := (index > group.first || line > 1) && (index < group.last || line == 0)
				if connect && strings.HasPrefix(entryRows[line], "   ") {
					entryRows[line] = " " + text("│", border) + entryRows[line][2:]
				}
			}
		}
		rows = append(rows, entryRows...)
		if inGroup && index == group.last {
			rows = append(rows, m.toolActivityControl(group, false, width)...)
		}
	}
	if m.cache != nil {
		*m.cache = transcriptCache{key: key, valid: true, rows: rows, starts: starts, entries: entries}
	}
	return rows, starts
}
func (m model) renderEntry(entry message, index, width int) []string {
	card := m
	card.sectionPrefix = m.sectionKey(index, "")
	if m.folded[index] {
		card.details = !card.details
	}
	var body string
	switch entry.role {
	case "YOU":
		content := lipgloss.NewStyle().Width(max(1, width-4)).Render(entry.text)
		body = box(content, width, lipgloss.Height(content)+2, m.composerBorderColor(), m.variant == 4)
	case "SOLOMON":
		content := renderMarkdown(entry.text, max(8, width-6))
		rows := strings.Split(content, "\n")
		for i := range rows {
			rows[i] = "   " + rows[i]
		}
		body = strings.Join(rows, "\n")
	case "TOOLCALL":
		body = card.toolCard(entry.text, width)
	case "TOOL":
		body = toolStatusIndicator("success") + " " + text(artifactTitle(entry.text), "#FFF69D")
	case "NOTICE":
		body = card.notification(entry.text, index, width)
	case "SUBAGENT":
		body = card.subagentCard(entry.text, width)
	case "MEDIA":
		body = imagePreview(width)
	default:
		title, content, found := strings.Cut(entry.text, "\n")
		if !found {
			content = title
			title = entry.role
		}
		body = card.artifact(entry.role, title, content, width)
		if entry.role == "SOURCE" {
			body = mockupSurface(body, width, lipgloss.Height(body))
		}
	}
	rendered := lipgloss.NewStyle().Width(width).Render(body)
	if entry.role != "YOU" && entry.role != "SOLOMON" && entry.role != "SOURCE" && entry.role != "MEDIA" && entry.role != "NOTICE" {
		lines := strings.Split(rendered, "\n")
		header := 0
		if entry.role == "TOOLCALL" && len(lines) > 1 {
			header = 1
		}
		lines[header] = clickZone(fmt.Sprintf("artifact:%d", index), lines[header])
		rendered = strings.Join(lines, "\n")
	}

	rows := strings.Split(rendered, "\n")
	if entry.role == "YOU" && index > 0 {
		rows = append([]string{""}, rows...)
	}
	return rows
}
func (m model) transcript(width, height int) string {
	rows, _ := m.transcriptLines(width)
	height = max(1, height)
	top := max(0, len(rows)-height-min(m.scroll, max(0, len(rows)-height)))
	return fit(strings.Join(rows[top:min(len(rows), top+height)], "\n"), width, height)
}
func (m model) jumpArtifact(direction int) model {
	if m.palette {
		return m
	}
	view := m.scrollView()
	target := 0
	if direction > 0 {
		target = view.limit
		for _, start := range view.starts {
			if start > view.top {
				target = start
				break
			}
		}
	} else {
		for _, start := range view.starts {
			if start >= view.top {
				break
			}
			target = start
		}
	}
	m.scroll = max(0, view.limit-min(view.limit, target))
	return m
}
