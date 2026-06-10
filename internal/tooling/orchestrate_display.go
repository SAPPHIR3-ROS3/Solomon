package tooling

import (
	"encoding/json"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const (
	orchestrateDisplayHeadTail = 25
	orchestrateDisplayMaxLines = orchestrateDisplayHeadTail * 2
	orchestrateTruncatedMarker = "...TRUNCATED..."
)

type orchestrateDisplayLine struct {
	num int
	src string
}

func formatOrchestrateToolDisplayLines(m map[string]json.RawMessage) []string {
	source := jsonDisplayString(m["source"])
	if source == "" {
		return []string{termcolor.OrchestrateToolHeaderLine()}
	}
	lines := splitEditContentLines(source)
	if len(lines) == 0 {
		return []string{termcolor.OrchestrateToolHeaderLine()}
	}
	width := orchestrateLineNumWidth(len(lines))
	out := []string{termcolor.OrchestrateToolHeaderLine()}
	for _, item := range orchestrateDisplayLines(lines) {
		if item.src == orchestrateTruncatedMarker {
			out = append(out, termcolor.WrapThinking(orchestrateTruncatedMarker))
			continue
		}
		out = append(out, formatOrchestrateSourceLine(item.num, item.src, width))
	}
	out = append(out, termcolor.OrchestrateCodeFooterLine())
	return out
}

func orchestrateLineNumWidth(totalLines int) int {
	if totalLines < 1 {
		return 1
	}
	return len(fmt.Sprintf("%d", totalLines))
}

func orchestrateDisplayLines(lines []string) []orchestrateDisplayLine {
	if len(lines) <= orchestrateDisplayMaxLines {
		out := make([]orchestrateDisplayLine, len(lines))
		for i, ln := range lines {
			out[i] = orchestrateDisplayLine{num: i + 1, src: ln}
		}
		return out
	}
	out := make([]orchestrateDisplayLine, 0, orchestrateDisplayMaxLines+1)
	for i, ln := range lines[:orchestrateDisplayHeadTail] {
		out = append(out, orchestrateDisplayLine{num: i + 1, src: ln})
	}
	out = append(out, orchestrateDisplayLine{src: orchestrateTruncatedMarker})
	tailStart := len(lines) - orchestrateDisplayHeadTail
	for i, ln := range lines[tailStart:] {
		out = append(out, orchestrateDisplayLine{num: tailStart + i + 1, src: ln})
	}
	return out
}

func formatOrchestrateSourceLine(num int, src string, width int) string {
	numPart := termcolor.WrapThinking(fmt.Sprintf("%*d", width, num))
	codePart := highlightGoLine(expandDisplayTabs(src))
	return numPart + " " + codePart
}
