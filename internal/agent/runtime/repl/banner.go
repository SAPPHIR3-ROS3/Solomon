package repl

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

var reStripANSI = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

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

func visibleCells(s string) int {
	return displayCells(reStripANSI.ReplaceAllString(s, ""))
}

// logoDisplayWidth calcola la larghezza di una riga del logo ignorando il padding
// a destra composto da Braille blank (U+2800) e spazi.
func logoDisplayWidth(plain string) int {
	trimmed := strings.TrimRightFunc(plain, func(r rune) bool {
		return r == '\u2800' || r == ' '
	})
	return displayCells(trimmed)
}

func borderPaint(s string) string {
	return termcolor.WrapBoldGold(s)
}

func PrintWelcomeBanner(out io.Writer, cfg *config.Root, model, projHex, projRoot string, replShellFirst bool, updateNotice *updater.Notice) {
	welcomeOut := termcolor.WrapWhite("Welcome to ") + termcolor.WrapBoldGold("Solomon")
	wWel := visibleCells(welcomeOut)
	logoLines := logo.WelcomeLogoLines()
	var logoW int
	for _, ln := range logoLines {
		if w := logoDisplayWidth(ln.Plain); w > logoW {
			logoW = w
		}
	}
	gap := 2
	colLeftW := logoW + gap
	if colLeftW < 1+wWel {
		colLeftW = 1 + wWel
	}
	nChats, uSum, rSum, sSum, _ := chatstore.ProjectWelcomeStats(projHex)
	skillN, _ := skills.InstalledSkillCount(projHex, projRoot)
	var right []string
	right = append(right, termcolor.WrapBoldGold(commands.VersionString()))
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
	mcpN, _ := solomonmcp.ConfiguredServerCount()
	if mcpN == 1 {
		right = append(right, "1 MCP")
	} else {
		right = append(right, fmt.Sprintf("%d MCP", mcpN))
	}
	right = append(right, "/connect to link new providers")
	right = append(right, "/models to switch models")
	right = append(right, "/help to show available commands")
	if replShellFirst {
		right = append(right, "!<prompt> to send input to the assistant")
	} else {
		right = append(right, "!<command> to execute commands on the shell")
	}
	if runtime.GOOS == "windows" {
		right = append(right, "Ctrl+V to paste a clipboard image")
	}
	maxRightW := 0
	for _, ln := range right {
		if w := visibleCells(ln); w > maxRightW {
			maxRightW = w
		}
	}
	colRightW := maxRightW
	if colRightW < 1 {
		colRightW = 1
	}
	innerW := colLeftW + 1 + colRightW
	if innerW < wWel+2 {
		innerW = wWel + 2
	}
	colRightW = innerW - colLeftW - 1
	if colRightW < 1 {
		colRightW = 1
		innerW = colLeftW + 1 + colRightW
	}
	padL := colLeftW - 1 - wWel
	if padL < 0 {
		padL = 0
	}
	fmt.Fprintln(out, borderPaint("┌")+borderPaint("─")+welcomeOut+borderPaint(strings.Repeat("─", padL)+"┬"+strings.Repeat("─", colRightW)+"┐"))
	maxH := len(logoLines)
	if len(right) > maxH {
		maxH = len(right)
	}
	for i := 0; i < maxH; i++ {
		left := ""
		lw := 0
		if i < len(logoLines) {
			left = logoLines[i].ANSI
			lw = logoDisplayWidth(logoLines[i].Plain)
		}
		rpart := ""
		if i < len(right) {
			rpart = right[i]
		}
		lpad := colLeftW - lw
		if lpad < 0 {
			lpad = 0
		}
		rw := visibleCells(rpart)
		rpad := colRightW - rw
		if rpad < 0 {
			rpad = 0
		}
		fmt.Fprintf(out, "%s%s%s%s%s%s%s\n", borderPaint("│"), left, strings.Repeat(" ", lpad), borderPaint("│"), rpart, strings.Repeat(" ", rpad), borderPaint("│"))
	}
	modelLine := termcolor.WrapAssistant("(not configured — run /onboard)")
	if strings.TrimSpace(model) != "" {
		eff := "none"
		fast := false
		if cfg != nil {
			eff = cfg.ReasoningEffortDisplayLabel()
			fast = cfg.FastModeEnabledForProvider(config.ProviderByName(cfg, cfg.Current.Provider))
		}
		modelLine = fmt.Sprintf("%s (%s)", termcolor.WrapAssistant(model), termcolor.WrapThinking(eff))
		if fast {
			modelLine += " " + termcolor.WrapThinking("(fast)")
		}
	}
	abs := projRoot
	if a, err := filepath.Abs(projRoot); err == nil {
		abs = a
	}
	br := cachedGitBranch(abs)
	pathLine := abs
	if br != "" {
		pathLine = fmt.Sprintf("%s (%s)", abs, br)
	}
	fmt.Fprintln(out, borderPaint("├"+strings.Repeat("─", colLeftW)+"┴"+strings.Repeat("─", colRightW)+"┤"))
	mpad := innerW - visibleCells(modelLine)
	if mpad < 0 {
		mpad = 0
	}
	fmt.Fprintf(out, "%s%s%s%s\n", borderPaint("│"), modelLine, strings.Repeat(" ", mpad), borderPaint("│"))
	ppad := innerW - visibleCells(pathLine)
	if ppad < 0 {
		ppad = 0
	}
	fmt.Fprintf(out, "%s%s%s%s\n", borderPaint("│"), pathLine, strings.Repeat(" ", ppad), borderPaint("│"))
	if updateNotice != nil {
		line1 := termcolor.WrapAssistant("Update available: ") + termcolor.WrapBoldGold(updateNotice.Latest)
		l1pad := innerW - visibleCells(line1)
		if l1pad < 0 {
			l1pad = 0
		}
		fmt.Fprintf(out, "%s%s%s%s\n", borderPaint("│"), line1, strings.Repeat(" ", l1pad), borderPaint("│"))
		hint := "Run /upgrade to install · /autoupdate on|off"
		if cfg != nil && cfg.AutoUpdateEnabled() {
			hint = "autoupdate=true — installing in background · /autoupdate off to disable"
		}
		cur := strings.TrimSpace(updateNotice.Current)
		if cur != "" {
			hint = fmt.Sprintf("%s (current %s)", hint, cur)
		}
		line2 := termcolor.WrapThinking(hint)
		l2pad := innerW - visibleCells(line2)
		if l2pad < 0 {
			l2pad = 0
		}
		fmt.Fprintf(out, "%s%s%s%s\n", borderPaint("│"), line2, strings.Repeat(" ", l2pad), borderPaint("│"))
	}
	fmt.Fprintln(out, borderPaint("└"+strings.Repeat("─", innerW)+"┘"))
}
