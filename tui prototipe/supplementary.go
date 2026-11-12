package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) supplementaryComponent(name string, width int) (string, bool) {
	sample := m
	sample.details = m.componentState != 1
	sample.sectionPrefix = fmt.Sprintf("component-%d.0.", m.componentIndex)
	section := func(title, body string) string {
		return sample.toolSection("MARKDOWN", title, body, max(8, width-6))
	}
	var body string
	switch name {
	case "File summary":
		body = m.fileSummary(width)
	case "Compaction":
		body = section("Context compacted", "Conversation context has been compacted.")
		if sample.details {
			body += "\n" + section("Summary", "## Current task\nAdd read-only image previews to the editor.\n\n- Preserve binary files\n- Keep text autosave unchanged\n- Verify PNG and JPEG previews")
			body += "\n" + section("Recent messages", "You: Verify image tabs do not trigger autosave.\n\nAssistant: Image previews use the binary endpoint and remain read-only.")
		}
	case "Reasoning":
		content := text("thought for 2s", muted)
		if m.componentState == 0 {
			copy := lipgloss.NewStyle().Width(max(1, width-6)).Render("I will inspect the editor routing, preserve the text path, and verify binary image handling.")
			content = text(copy, muted) + "\n" + content
		} else if m.componentState == 2 {
			content = reasoningMarker
		}
		body = clickZone("action:toggle_reasoning", content)
	case "Worked for":
		body = text("worked for 12s", gold)
		if m.componentState == 2 {
			body = workedForMarker
		}
	case "Subagent chat":
		body = m.subchatPreview(width)
	case "Subagent activity":
		label := runningSubagentMarker + "  " + bold("2", gold) + text(" subagents working", muted)
		if m.componentState == 1 {
			label += text("  ▾", muted)
			body = box(clickZone("action:toggle_subagent_activity", label), lipgloss.Width(label)+4, 3, border, true)
		} else {
			label += text("  ▴", muted)
			first := runningSubagentMarker + "  " + text("[gpt-6.1-sol] [high ", muted) + text("ϟ", gold) + text("]", muted) + "  " + text("Verify editor regressions", ink)
			second := runningSubagentMarker + "  " + text("[Claude Sonnet] [medium]", muted) + "  " + text("Review image accessibility", ink)
			content := clickZone("action:toggle_subagent_activity", label) + "\n" + clickZone("subchat:0", first) + "\n" + clickZone("subchat:1", second)
			body = box(content, max(8, width-6), 5, border, true)
		}
		if m.sections["preview.subagent.open"] {
			body = m.subchatPreview(width)
		}
	case "Rate limit":
		body = lipgloss.NewStyle().Width(max(1, width-6)).Height(3).Align(lipgloss.Center, lipgloss.Center).Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#a83b3b")).Bold(true).Render("Usage limit reached · Resets in 30s")
	case "Interrupted generation":
		body = text("■ Generation interrupted", "#ff626b")
	case "Mode switch":
		body = m.modeSwitchPreview(width)
	case "Edit diff":
		body = sample.artifact("DIFF", "EditorPage.tsx", "- return <CodeEditor file={file} />;\n+ if (isImageFile(file.path)) return <ImagePreview file={file} />;\n+ return <CodeEditor file={file} />;", max(8, width-6))
	case "Message actions":
		body = lipgloss.NewStyle().Width(max(1, width-6)).Align(lipgloss.Right).Render(text("Delete   Regenerate   Copy", muted))
	default:
		return "", false
	}
	rows := strings.Split(body, "\n")
	for i := range rows {
		rows[i] = "   " + rows[i]
	}
	return strings.Join(rows, "\n"), true
}
