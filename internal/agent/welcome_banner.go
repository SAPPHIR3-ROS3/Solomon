package agent

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/logo"
	"solomon/internal/skills"
	"solomon/internal/termcolor"
)

func gitBranch(dir string) string {
	c := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if out, err := c.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return ""
	}
	c2 := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := c2.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func displayCells(s string) int {
	n := 0
	for _, r := range s {
		n += runeDisplayWidth(r)
	}
	return n
}

func runeDisplayWidth(r rune) int {
	switch {
	case r == 0 || r < 0x20:
		return 0
	case (r >= 0x1100 && r <= 0x115F) || (r >= 0x2329 && r <= 0x232A):
		return 2
	case r >= 0x2E80 && r <= 0x303E:
		return 2
	case r >= 0x3040 && r <= 0x3247:
		return 2
	case r >= 0x3250 && r <= 0x4DBF:
		return 2
	case r >= 0x4E00 && r <= 0x9FFF:
		return 2
	case r >= 0xA000 && r <= 0xA4C6:
		return 2
	case r >= 0xAC00 && r <= 0xD7A3:
		return 2
	case r >= 0xF900 && r <= 0xFAFF:
		return 2
	case r >= 0xFE10 && r <= 0xFE19:
		return 2
	case r >= 0xFE30 && r <= 0xFE6F:
		return 2
	case r >= 0xFF00 && r <= 0xFF60:
		return 2
	case r >= 0xFFE0 && r <= 0xFFE6:
		return 2
	case r >= 0x20000 && r <= 0x2FFFD:
		return 2
	case r >= 0x30000 && r <= 0x3FFFD:
		return 2
	default:
		return 1
	}
}

func printWelcomeBanner(out io.Writer, cfg *config.Root, model, projHex, projRoot string) {
	fmt.Fprintf(out, "%s\n\n", termcolor.WrapWhite("Welcome to ")+termcolor.WrapBoldGold("Solomon"))
	raw := strings.ReplaceAll(logo.ASCII, "\r\n", "\n")
	logoLines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var logoW int
	for _, ln := range logoLines {
		if w := displayCells(ln); w > logoW {
			logoW = w
		}
	}
	gap := 4
	rightColStart := logoW + gap
	nChats, uSum, rSum, sSum, _ := chatstore.ProjectWelcomeStats(projHex)
	skillN, _ := skills.InstalledSkillCount(projHex, projRoot)
	var right []string
	const resumeLine = "/resume to show most recent chats"
	if nChats == 1 {
		right = append(right, "1 chat  "+resumeLine)
	} else {
		right = append(right, fmt.Sprintf("%d chats  %s", nChats, resumeLine))
	}
	totalDisp := uSum + rSum + sSum
	tokLine := termcolor.WelcomeUsageTotals(uSum, rSum, sSum, totalDisp)
	right = append(right, tokLine+"  token across chats for this path")
	right = append(right, fmt.Sprintf("%d skills", skillN))
	right = append(right, "0 MCP (soon)")
	right = append(right, "/connect to link new providers")
	right = append(right, "/models to switch models")
	right = append(right, "/help to show available commands")
	right = append(right, "!<command> to execute commands on the shell")
	maxH := len(logoLines)
	if len(right) > maxH {
		maxH = len(right)
	}
	for i := 0; i < maxH; i++ {
		left := ""
		if i < len(logoLines) {
			left = logoLines[i]
		}
		rpart := ""
		if i < len(right) {
			rpart = right[i]
		}
		pad := rightColStart - displayCells(left)
		if pad < gap {
			pad = gap
		}
		fmt.Fprintln(out, left+strings.Repeat(" ", pad)+rpart)
	}
	eff := "none"
	if cfg != nil {
		if s := strings.TrimSpace(cfg.ReasoningEffort); s != "" {
			eff = s
		} else if lbl := cfg.ReasoningEffortLabel(); lbl != "" {
			eff = lbl
		}
	}
	fmt.Fprintf(out, "\n%s (%s)\n", termcolor.WrapAssistant(model), termcolor.WrapThinking(eff))
	abs := projRoot
	if a, err := filepath.Abs(projRoot); err == nil {
		abs = a
	}
	br := gitBranch(abs)
	if br != "" {
		fmt.Fprintf(out, "%s (%s)\n", abs, br)
	} else {
		fmt.Fprintln(out, abs)
	}
}

