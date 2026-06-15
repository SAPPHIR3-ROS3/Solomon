package commands

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func Registry(cfg *config.Root) [][]string {
	tab := getSlashBuiltins()
	rows := make([][]string, 0, len(tab)+1)
	for i := range tab {
		b := &tab[i]
		if !slashVisible(b, cfg) {
			continue
		}
		rows = append(rows, []string{b.helpCol, b.detail})
	}
	rows = append(rows, []string{"/skill:<name>", "/skill:<name> [request] — force one installed skill into this turn; names may contain spaces; keeps /skill:... visible in chat"})
	return rows
}

func WriteHelp(w io.Writer, projHex, projRoot string, cfg *config.Root) {
	rows := Registry(cfg)
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	maxCmd := 0
	for _, row := range rows {
		if n := len(row[0]); n > maxCmd {
			maxCmd = n
		}
	}
	var buf bytes.Buffer
	for _, row := range rows {
		fmt.Fprintf(&buf, "%-*s\t%s\n", maxCmd, row[0], row[1])
	}
	skills.WriteSkillInstallHelpSection(&buf, maxCmd)
	_ = skills.WriteSkillsHelpSection(&buf, maxCmd, projHex, projRoot)
	writePromptTemplatesHelpSection(&buf)
	termcolor.WriteSystem(w, buf.String())
}

func writePromptTemplatesHelpSection(w io.Writer) {
	tplDir, err := paths.PromptTemplatesDir()
	if err != nil {
		return
	}
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Prompt templates")
	fmt.Fprintf(w, "  %s/*.tmpl\tagent, chat, title, summarize, images, …\n", tplDir)
	fmt.Fprintf(w, "  %s\t[prompt_templates] SHA256 after you accept edits in an interactive session\n", cfgPath)
	fmt.Fprintln(w, "  On startup, modified templates prompt yes(y) no(n) acceptAll(a) denyAll(d); deny resets to embedded default.")
	fmt.Fprintln(w, "  make install runs solomon templates install (SHA check before writing files).")
	fmt.Fprintln(w, "  Pipes/scripts/CI without a TTY exit with an error — run interactively or align [prompt_templates] SHAs in config.")
}
