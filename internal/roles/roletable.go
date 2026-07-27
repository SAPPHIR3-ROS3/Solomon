package roles

import (
	"fmt"
	"strings"
)

func padCell(s string, width int) string {
	if width <= 0 {
		return s
	}
	n := visibleLen(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func visibleLen(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r == '\t':
			n += 4
		case r < 0x20, r == 0xFE0E, r == 0xFE0F, r == 0x200D:
			continue
		default:
			n += runeCellWidth(r)
		}
	}
	return n
}

func runeCellWidth(r rune) int {
	switch {
	case r >= 0x1F000 && r <= 0x1FFFF:
		return 2
	case r >= 0x2300 && r <= 0x23FF:
		return 2
	case r >= 0x2600 && r <= 0x27BF:
		return 2
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

func colWidth(header string, cells []string) int {
	w := visibleLen(header)
	for _, c := range cells {
		if n := visibleLen(c); n > w {
			w = n
		}
	}
	return w
}

func FormatSubagentTable(view TableView) string {
	return formatTable(view, false)
}

func FormatCompactTable(view TableView) string {
	return formatTable(view, true)
}

const maxBenchmarkPreviewRows = 10

func formatTable(view TableView, compact bool) string {
	if len(view.Columns) == 0 {
		return ""
	}
	modelCells := make([]string, len(view.Rows))
	providerCells := make([]string, len(view.Rows))
	colCells := make([][]string, len(view.Columns))
	for i := range view.Columns {
		colCells[i] = make([]string, len(view.Rows))
	}
	for i, row := range view.Rows {
		modelCells[i] = row.Model
		providerCells[i] = "[" + row.Provider + "]"
		for j, ch := range view.Columns {
			if v, ok := row.Scores[ch]; ok {
				colCells[j][i] = fmt.Sprintf("%d", v)
			} else {
				colCells[j][i] = "—"
			}
		}
	}
	modelW := colWidth("model", modelCells)
	if !compact {
		modelW = colWidth("model", append(modelCells, providerCells...))
	}
	colWidths := make([]int, len(view.Columns))
	for j, ch := range view.Columns {
		colWidths[j] = colWidth(CharacteristicColumn(ch), colCells[j])
		if colWidths[j] < 1 {
			colWidths[j] = 1
		}
	}
	var b strings.Builder
	if legend := CharacteristicLegend(view.Columns); legend != "" {
		b.WriteString(legend)
		b.WriteString("\n")
	}
	sep := func() {
		b.WriteString("├")
		b.WriteString(strings.Repeat("─", modelW+2))
		for _, w := range colWidths {
			b.WriteString("┼")
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString("┤\n")
	}
	top := func() {
		b.WriteString("┌")
		b.WriteString(strings.Repeat("─", modelW+2))
		for _, w := range colWidths {
			b.WriteString("┬")
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString("┐\n")
	}
	bottom := func() {
		b.WriteString("└")
		b.WriteString(strings.Repeat("─", modelW+2))
		for _, w := range colWidths {
			b.WriteString("┴")
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString("┘\n")
	}
	writeRow := func(cells []string, widths []int) {
		b.WriteString("│ ")
		b.WriteString(padCell(cells[0], widths[0]))
		b.WriteString(" ")
		for i := 1; i < len(cells); i++ {
			b.WriteString("│ ")
			b.WriteString(padCell(cells[i], widths[i]))
			b.WriteString(" ")
		}
		b.WriteString("│\n")
	}
	top()
	header := make([]string, 1+len(view.Columns))
	header[0] = "model"
	for i, ch := range view.Columns {
		header[i+1] = CharacteristicColumn(ch)
	}
	widths := append([]int{modelW}, colWidths...)
	writeRow(header, widths)
	sep()
	for i := range view.Rows {
		row := make([]string, 1+len(view.Columns))
		row[0] = modelCells[i]
		for j := range view.Columns {
			row[j+1] = colCells[j][i]
		}
		writeRow(row, widths)
		if !compact && view.Rows[i].Provider != "" {
			pRow := make([]string, 1+len(view.Columns))
			pRow[0] = providerCells[i]
			for j := range view.Columns {
				pRow[j+1] = ""
			}
			writeRow(pRow, widths)
		}
		if !compact && i < len(view.Rows)-1 {
			sep()
		}
	}
	bottom()
	return b.String()
}
