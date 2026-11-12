package main

import (
	"fmt"
	"strings"
	"time"
)

var notificationStart = time.Now()

func (m model) notificationActive() bool {
	return m.componentOpen && componentNames[m.componentIndex%len(componentNames)] == "Notification"
}

func notificationProgress(width int) string {
	return fmt.Sprintf("\x1b]777;notice-progress-%d\x1b\\", width) + strings.Repeat("─", width)
}

func animateNotification(view string) string {
	if !strings.Contains(view, "notice-progress-") {
		return view
	}
	progress := min(1.0, max(0.0, time.Since(notificationStart).Seconds()/10))
	for width := 6; width <= 42; width++ {
		marker := notificationProgress(width)
		if !strings.Contains(view, marker) {
			continue
		}
		position := progress * float64(width)
		filled := min(width, int(position))
		bar := text(strings.Repeat("─", filled), activeInterface.Colors["accent"])
		if filled < width {
			fraction := position - float64(filled)
			baseR, baseG, baseB := parseHexRGB(strings.TrimPrefix(border, "#"))
			accentR, accentG, accentB := parseHexRGB(strings.TrimPrefix(activeInterface.Colors["accent"], "#"))
			mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*fraction) }
			color := fmtColor(mix(baseR, accentR), mix(baseG, accentG), mix(baseB, accentB))
			bar += text("─", color) + text(strings.Repeat("─", width-filled-1), border)
		}
		view = strings.ReplaceAll(view, marker, bar)
	}
	return view
}
