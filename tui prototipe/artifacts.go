package main

import (
	"fmt"
	"strconv"
	"strings"

	sharedsyntax "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
	"github.com/charmbracelet/lipgloss"
)

func artifactColor(role string) string {
	switch role {
	case "CODE", "QUERY":
		return green
	case "OUTPUT", "SOURCE":
		return muted
	case "ERROR":
		return "#FF5555"
	case "DIFF":
		return violet
	default:
		return gold
	}
}

func (m model) artifact(role, title, body string, width int) string {
	title = artifactTitle(title)
	color := artifactColor(role)
	width = max(8, width)
	inner := width - 4
	tl, tr, bl, br, horizontal, vertical := "╭", "╮", "╰", "╯", "─", "│"
	frameLine := func(content string) string {
		return text(vertical, color) + " " + fit(content, inner, 1) + " " + text(vertical, color)
	}
	rows := []string{}
	caption := " " + title + " "
	if lipgloss.Width(caption) <= width-3 {
		rows = append(rows, text(tl+horizontal+caption+strings.Repeat(horizontal, width-3-lipgloss.Width(caption))+tr, color))
	} else {
		rows = append(rows, text(tl+strings.Repeat(horizontal, width-2)+tr, color))
		for _, part := range highlightedWrappedLine(bold(title, color), inner) {
			rows = append(rows, frameLine(part))
		}
	}
	content := strings.Split(body, "\n")
	if !m.details {
		summary := fmt.Sprintf("▸ %d lines · %s expand", len(content), configuredHint("global", "toggle_details"))
		for _, part := range highlightedWrappedLine(text(summary, muted), inner) {
			rows = append(rows, frameLine(part))
		}
	} else if role == "CODE" {
		highlighted := highlightGo(sharedsyntax.FormatGo(body))
		digits := max(3, len(strconv.Itoa(len(highlighted))))
		for index, row := range highlighted {
			label := fmt.Sprintf("%*d ", digits, index+1)
			available := max(1, inner-len(label))
			for continuation, part := range highlightedCodeLine(row, available) {
				gutter := label
				if continuation > 0 {
					gutter = strings.Repeat(" ", len(label))
				}
				rows = append(rows, frameLine(text(gutter, muted)+part))
			}
		}
	} else {
		if role == "MARKDOWN" || role == "PLAN" || role == "REASONING" {
			content = strings.Split(renderMarkdown(body, inner), "\n")
		}
		for _, row := range content {
			for _, part := range highlightedWrappedLine(row, inner) {
				inkColor := ink
				if role == "OUTPUT" || role == "SOURCE" {
					inkColor = muted
				}
				if role == "DIFF" && strings.HasPrefix(part, "+") {
					inkColor = green
				}
				if role == "DIFF" && strings.HasPrefix(part, "-") {
					inkColor = "#FF5555"
				}
				if !strings.ContainsRune(part, 27) {
					part = text(part, inkColor)
				}
				rows = append(rows, frameLine(part))
			}
		}
	}
	rows = append(rows, text(bl+strings.Repeat(horizontal, width-2)+br, color))
	return strings.Join(rows, "\n")
}
