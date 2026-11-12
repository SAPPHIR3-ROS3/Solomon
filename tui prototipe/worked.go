package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	sharedsyntax "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
)

const workedForMarker = "\x1b]777;worked-for\x1b\\worked for 0s"
const reasoningMarker = "\x1b]777;reasoning-wave\x1b\\thought for 0s"

var workedForStart = time.Now()

func (m model) workedForActive() bool {
	name := componentNames[m.componentIndex%len(componentNames)]
	return m.componentOpen && (name == "Worked for" || name == "Reasoning") && m.componentState == 2
}

func resetWorkedFor() { workedForStart = time.Now() }

func workedDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remaining := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, remaining)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, remaining)
	}
	return fmt.Sprintf("%ds", remaining)
}

func animateWorkedFor(view string) string {
	if !strings.Contains(view, workedForMarker) && !strings.Contains(view, reasoningMarker) {
		return view
	}
	elapsed := time.Since(workedForStart)
	marker := workedForMarker
	label := "worked for " + workedDuration(int(elapsed.Seconds()))
	if strings.Contains(view, reasoningMarker) {
		marker = reasoningMarker
		label = "thought for " + workedDuration(int(elapsed.Seconds()))
	}
	chars := []rune(label)
	phase := float64(elapsed.Milliseconds()%1600) / 1600
	center := -3 + phase*float64(len(chars)+6)
	var result strings.Builder
	for i, ch := range chars {
		distance := math.Abs(float64(i) - center)
		strength := math.Max(0, 1-distance/3.0)
		strength = strength * strength * (3 - 2*strength)
		baseR, baseG, baseB := parseHexRGB(strings.TrimPrefix(muted, "#"))
		lightR, lightG, lightB := parseHexRGB(strings.TrimPrefix(ink, "#"))
		mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*strength)) }
		color := fmtColor(mix(baseR, lightR), mix(baseG, lightG), mix(baseB, lightB))
		result.WriteString(sharedsyntax.ANSI(string(ch), color))
	}
	return strings.ReplaceAll(view, marker, result.String())
}
