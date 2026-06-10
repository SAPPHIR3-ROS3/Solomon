package tooling

import (
	"encoding/json"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func formatSwitchModeToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{termcolor.SwitchModeToolHeaderLine(switchModeDisplayLabel(jsonDisplayString(m["mode"])))}
}

func switchModeDisplayLabel(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "Agent"
	}
	return strings.ToUpper(mode[:1]) + mode[1:]
}
