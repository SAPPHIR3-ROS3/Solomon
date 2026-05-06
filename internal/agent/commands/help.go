package commands

import (
	"fmt"
	"io"
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

func Registry() [][]string {
	tab := getSlashBuiltins()
	rows := make([][]string, len(tab))
	for i := range tab {
		b := &tab[i]
		rows[i] = []string{b.helpCol, b.detail}
	}
	return rows
}

func WriteHelp(w io.Writer, projHex, projRoot string) {
	rows := Registry()
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	maxCmd := 0
	for _, row := range rows {
		if n := len(row[0]); n > maxCmd {
			maxCmd = n
		}
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s\t%s\n", maxCmd, row[0], row[1])
	}
	_ = skills.WriteSkillsHelpSection(w, maxCmd, projHex, projRoot)
}
