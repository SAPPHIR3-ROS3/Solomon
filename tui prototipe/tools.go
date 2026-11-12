package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type recordedTool struct {
	Status    string `json:"status,omitempty"`
	Name      string
	Arguments json.RawMessage
	Result    string
}

func decodeResult(raw string) map[string]any {
	var result map[string]any
	if json.Unmarshal([]byte(raw), &result) != nil {
		return nil
	}
	return result
}
func resultOutput(raw string) string {
	for depth := 0; depth < 4; depth++ {
		value := decodeResult(raw)
		if value == nil {
			return raw
		}
		if out, ok := value["output"].(string); ok {
			raw = out
			continue
		}
		if message, ok := value["error"].(string); ok {
			return message
		}
		return readableFields(value)
	}
	return raw
}
func readableFields(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := []string{}
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			rows = append(rows, key+": "+value)
		case []any:
			rows = append(rows, key+":")
			for _, item := range value {
				if record, ok := item.(map[string]any); ok {
					rows = append(rows, readableFields(record))
				} else {
					rows = append(rows, fmt.Sprint(item))
				}
			}
		case map[string]any:
			rows = append(rows, key+":\n"+readableFields(value))
		default:
			rows = append(rows, fmt.Sprintf("%s: %v", key, value))
		}
	}
	return strings.Join(rows, "\n")
}
func (m model) toolCard(raw string, width int) string {
	width = max(8, width-8)
	var tool recordedTool
	if err := json.Unmarshal([]byte(raw), &tool); err != nil {
		return text("Invalid recorded tool: "+err.Error(), "#FF5555")
	}
	args := decodeResult(string(tool.Arguments))
	intent, _ := args["intent"].(string)
	marker := "▾"
	if !m.details {
		marker = "▸"
	}
	header := bold(intent, gold) + text("  "+marker, muted)
	if tool.Name == "orchestrate" {
		header = bold("CODE", cyan) + "  " + header
	}
	rows := []string{lipgloss.NewStyle().Width(width).Render(header)}
	if !m.details {
		return insetToolCard(rows, width, toolStatusIndicator(tool.Status))
	}
	rows = append(rows, bold("Tool: "+tool.Name, "#FFF69D"))
	if tool.Name == "searchTools" {
		query, _ := args["query"].(string)
		rows = append(rows, m.toolSection("QUERY", "Query", query, width))
		result := decodeResult(tool.Result)
		if result == nil {
			rows = append(rows, m.toolSection("OUTPUT", "Result", resultOutput(tool.Result), width))
			return insetToolCard(rows, width, toolStatusIndicator(tool.Status))
		}
		tools, _ := result["tools"].([]any)
		matches := []string{}
		for _, item := range tools {
			match, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := match["name"].(string)
			description, _ := match["description"].(string)
			signature, _ := match["sdk_call"].(string)
			matches = append(matches, bold(name, cyan), text(description, ink))
			if signature != "" {
				matches = append(matches, text(signature, muted))
			}
		}
		if len(tools) == 0 {
			matches = append(matches, text("No matching tools", muted))
		}
		rows = append(rows, m.toolSection("OUTPUT", "Result", strings.Join(matches, "\n"), width, fmt.Sprintf("%d tool matches", len(tools))))
		return insetToolCard(rows, width, toolStatusIndicator(tool.Status))
	}
	execution := ""
	if result := decodeResult(tool.Result); result != nil {
		label := "SDK calls —"
		if count, ok := result["tool_calls"]; ok {
			label = fmt.Sprintf("SDK calls %v", count)
		}
		if duration, ok := result["duration_ms"].(float64); ok {
			if elapsed := formatToolDuration(duration); elapsed != "" {
				label += " · " + elapsed
			}
		}
		execution = text(label, muted)
	}
	if tool.Name == "orchestrate" && execution == "" {
		execution = text("SDK calls —", muted)
	}
	if tool.Name == "orchestrate" {
		if !m.details {
			return insetToolCard([]string{header}, width, toolStatusIndicator(tool.Status))
		}
		if execution != "" {
			rows = append(rows, execution)
		}
		source, _ := args["source"].(string)
		rows = append(rows, m.toolSection("CODE", "Source", source, width))
	} else {
		params := map[string]any{}
		for key, value := range args {
			if key != "intent" {
				params[key] = value
			}
		}
		if len(params) > 0 {
			rows = append(rows, text(readableFields(params), ink))
		}
	}
	if tool.Result != "" {
		if tool.Name != "orchestrate" && execution != "" {
			rows = append(rows, execution)
		}
		rows = append(rows, m.toolSection("OUTPUT", "Result", resultOutput(tool.Result), width))
	}
	return insetToolCard(rows, width, toolStatusIndicator(tool.Status))
}

func formatToolDuration(milliseconds float64) string {
	if math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) || milliseconds < 0 {
		return ""
	}
	seconds := milliseconds / 1000
	precision := 2
	if seconds >= 10 {
		precision = 1
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(seconds, 'f', precision, 64), "0"), ".") + "s"
}

func insetToolCard(rows []string, width int, status string) string {
	frame := []string{"   " + text("╭"+strings.Repeat("─", width)+"╮", border)}
	first := true
	for _, row := range rows {
		for _, line := range highlightedWrappedLine(row, max(1, width)) {
			gutter := "   "
			if first {
				gutter = " " + status + " "
				first = false
			}
			frame = append(frame, gutter+text("│", border)+fit(line, width, 1)+text("│", border))
		}
	}
	frame = append(frame, "   "+text("╰"+strings.Repeat("─", width)+"╯", border))
	return strings.Join(frame, "\n")
}
