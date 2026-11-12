package main

import (
	"strings"
	"time"

	sharedsyntax "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
)

const runningToolMarker = "\x1b]777;tool-running\x1b\\·"

func toolStatusIndicator(status string) string {
	if status == "running" {
		return runningToolMarker
	}
	if status == "error" || status == "interrupted" {
		return sharedsyntax.ANSI("●", "#a83b3b")
	}
	return sharedsyntax.ANSI("●", activeInterface.Colors["tool_success"])
}

const runningSubagentMarker = "\x1b]777;subagent-running\x1b\\·"

func animateToolIndicators(view string) string {
	if !strings.Contains(view, runningToolMarker) && !strings.Contains(view, runningSubagentMarker) {
		return view
	}
	dot := "●"
	if time.Now().UnixMilli()/400%2 == 1 {
		dot = "○"
	}
	view = strings.ReplaceAll(view, runningToolMarker, sharedsyntax.ANSI(dot, activeInterface.Colors["accent"]))
	return strings.ReplaceAll(view, runningSubagentMarker, sharedsyntax.ANSI(dot, gold))
}
