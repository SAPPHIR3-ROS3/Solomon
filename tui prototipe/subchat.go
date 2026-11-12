package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) subchatContent(width int) (string, []string) {
	width = max(12, width-6)
	inner := width - 4
	task := "Verify image tabs do not trigger autosave."
	message := "Image previews do not enter the text autosave path. Checks passed."
	intent := "Inspect image preview autosave behavior"
	metadata := text("[gpt-6.1-sol] [high ", muted) + text("ϟ", gold) + text("]", muted)
	if m.sections["preview.subagent.1"] {
		task = "Review image accessibility."
		message = "Checking alternative text and keyboard navigation."
		intent = "Inspect image accessibility"
		metadata = text("[Claude Sonnet] [medium]", muted)
	}
	header := runningSubagentMarker + " " + bold("Subagent chat", ink) + "  " + metadata
	closeButton := clickZone("action:close_subchat_preview", text("[ × ]", muted))
	heading := spacedPair(header, closeButton, inner)
	prompt := lipgloss.NewStyle().Width(max(1, inner-4)).Render(task)
	prompt = box(prompt, inner, lipgloss.Height(prompt)+2, m.composerBorderColor(), true)
	args, _ := json.Marshal(map[string]string{"intent": intent, "source": "package main\n\nfunc main() {\n  println(42)\n}"})
	result, _ := json.Marshal(map[string]any{"tool_calls": 1, "duration_ms": 240, "output": "Image preview implementation inspected."})
	tool, _ := json.Marshal(recordedTool{Status: "success", Name: "orchestrate", Arguments: args, Result: string(result)})
	card := m
	card.details = true
	card.sectionPrefix = fmt.Sprintf("component-%d.subchat.", m.componentIndex)
	toolBody := card.toolCard(string(tool), inner)
	answer := lipgloss.NewStyle().Width(max(1, inner-6)).Render(message)
	answerRows := strings.Split(answer, "\n")
	for i := range answerRows {
		answerRows[i] = "   " + text(answerRows[i], ink)
	}
	body := strings.Join([]string{prompt, toolBody, strings.Join(answerRows, "\n")}, "\n")
	return heading, strings.Split(body, "\n")
}

func (m model) subchatScrollLimit(width int) int {
	_, rows := m.subchatContent(width)
	return max(0, len(rows)-max(1, (m.height-3)/2-3))
}

func (m model) subchatPreview(width int) string {
	heading, rows := m.subchatContent(width)
	available := max(1, (m.height-3)/2-3)
	top := min(max(0, m.subchatScroll), max(0, len(rows)-available))
	end := min(len(rows), top+available)
	body := heading + "\n" + strings.Join(rows[top:end], "\n")
	return box(body, max(12, width-6), end-top+3, border, true)
}
