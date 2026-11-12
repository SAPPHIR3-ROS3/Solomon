package test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestMissionMatchesGUIHomeStructure(t *testing.T) {
	strip := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, size := range [][2]int{{42, 24}, {80, 24}, {120, 40}, {180, 60}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			cmd := exec.Command("../mission", "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1]))
			cmd.Env = append(os.Environ(), "SOLOMON_PREVIEW_NAME=Test User")
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			plain := strip.ReplaceAllString(string(out), "")
			if strings.Contains(plain, "Welcome to Solomon") {
				t.Fatal("Mission is still using the classic framed welcome")
			}
			if !strings.Contains(plain, "Welcome back, Test User") {
				t.Fatal("missing GUI greeting overlay")
			}
			rows := strings.Split(plain, "\n")
			top, bottom, workspace, toolbar, branch, stats := -1, -1, -1, -1, -1, -1
			for i, row := range rows {
				if strings.Contains(row, "╭") {
					top = i
				}
				if strings.Contains(row, "╰") {
					bottom = i
				}
				if strings.Contains(row, "Solomon ▾") {
					workspace = i
				}
				if strings.Contains(row, "None ▾") && strings.Contains(row, "Agent") {
					toolbar = i
				}
				if strings.Contains(row, "worktree ▾") {
					branch = i
				}
				if strings.Contains(row, "new thread") {
					stats = i
				}
			}
			if top < 0 || bottom <= top || workspace != top-1 {
				t.Fatalf("workspace/composer geometry: %d %d %d", workspace, top, bottom)
			}
			if toolbar <= top || toolbar >= bottom {
				t.Fatal("model / reasoning / Agent toolbar must be inside composer")
			}
			if branch != bottom+1 || stats != branch+1 {
				t.Fatal("branch and stats must follow composer")
			}
			if lipgloss.Width(strings.TrimRight(rows[top], " ")) != lipgloss.Width(strings.TrimRight(rows[bottom], " ")) {
				t.Fatal("composer edges are not aligned")
			}
			workspaceLeft := strings.Index(rows[workspace], "Solomon")
			composerLeft := strings.Index(rows[top], "╭")
			if workspaceLeft != composerLeft {
				t.Errorf("workspace left %d differs from composer %d", workspaceLeft, composerLeft)
			}
			if size[1] >= 32 && top <= size[1]/3 {
				t.Fatal("GUI group is not centered vertically")
			}
		})
	}
}
