package test

import (
	"github.com/charmbracelet/lipgloss"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestToolCardUsesDedicatedStatusColumn(t *testing.T) {
	out, err := exec.Command("../laboratory", "--snapshot", "120", "100", "tool_call").Output()
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile("\\x1b\\[[0-9;]*m").ReplaceAllString(string(out), "")
	rows := strings.Split(plain, "\n")
	header, intent, tool, source := -1, -1, -1, -1
	for i, row := range rows {
		if strings.Contains(row, "Leggere editor e API dei file") {
			header = i
		}
		if strings.Contains(row, "Tool: orchestrate") {
			tool = i
		}
		if strings.Contains(row, "Source") {
			source = i
		}
		if strings.Contains(row, "SDK calls") {
			intent = i
		}
	}
	if header < 0 || tool < 0 || source < 0 {
		t.Fatalf("tool card missing: header=%d tool=%d source=%d", header, tool, source)
	}
	marker := strings.Index(rows[header], "●")
	if marker < 0 {
		marker = strings.Index(rows[header], "○")
	}
	intentStart := strings.Index(rows[header], "CODE")
	column, width := -1, 0
	if marker >= 0 && intentStart >= marker {
		column = lipgloss.Width(rows[header][:marker])
		width = lipgloss.Width(rows[header][:intentStart])
	}
	if column != 2 || width != column+3 {
		t.Fatalf("status/int columns %d/%d cells; %q", column, width, rows[header])
	}
	for _, index := range []int{tool, source} {
		if strings.TrimSpace(rows[index]) == "" {
			t.Fatal("empty content row")
		}
		if strings.HasPrefix(strings.TrimLeft(rows[index], " "), "┌") {
			t.Fatal("card artifact starts at the status column")
		}
	}
	if intent >= 0 && intent <= header {
		t.Fatal("execution metadata should follow the tool header")
	}
}
