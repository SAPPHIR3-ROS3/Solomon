package test

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestArtifactFramesAreClosed(t *testing.T) {
	strip := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, width := range []string{"42", "80", "120"} {
			out, err := exec.Command("../"+name, "--snapshot", width, "200", "tool_call").Output()
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strip.ReplaceAllString(string(out), ""), "\n")
			for _, title := range []string{"Source", "Result"} {
				top := -1
				for i, row := range rows {
					if strings.Contains(row, title) && strings.ContainsAny(row, "┌╭┏") {
						top = i
						break
					}
				}
				if top < 0 {
					t.Fatalf("%s width %s missing %s frame", name, width, title)
				}
				right := "┐"
				vertical := "│"
				bottom := "└"
				bottomRight := "┘"
				if name == "orbit" {
					right = "╮"
					bottom = "╰"
					bottomRight = "╯"
				}
				if name == "mission" {
					right = "┓"
					vertical = "┃"
					bottom = "┗"
					bottomRight = "┛"
				}
				header := strings.TrimSpace(rows[top])
				if !strings.HasSuffix(header, right) {
					t.Errorf("%s %s: top border is open", name, title)
				}
				cells := lipgloss.Width(header)
				closed := false
				for _, row := range rows[top+1:] {
					row = strings.TrimSpace(row)
					if strings.HasPrefix(row, bottom) {
						if !strings.HasSuffix(row, bottomRight) {
							t.Errorf("%s %s: bottom border is open", name, title)
						}
						if lipgloss.Width(row) != cells {
							t.Errorf("%s %s: bottom edge misaligned", name, title)
						}
						closed = true
						break
					}
					if !strings.HasPrefix(row, vertical) || !strings.HasSuffix(row, vertical) {
						t.Errorf("%s %s: missing side border in %q", name, title, row)
					}
					if lipgloss.Width(row) != cells {
						t.Errorf("%s %s: side edges misaligned", name, title)
					}
				}
				if !closed {
					t.Errorf("%s %s: missing bottom edge", name, title)
				}
			}
		}
	}
}
