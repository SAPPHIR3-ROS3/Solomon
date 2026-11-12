package main

import (
	"strings"
	"time"
)

const modeProgressMarker = "\x1b]777;mode-progress\x1b\\------------------------"

func (m model) modeSwitchPreview(width int) string {
	width = max(40, width-6)
	inner := width - 4
	heading := bold("♛", gold) + " " + bold("Switching to Agent mode", ink)
	cancel := clickZone("action:cancel_mode_preview", text("[ Stay in Chat ]", muted))
	if m.componentState == 1 {
		return box(text("Stayed in Chat", muted), width, 3, border, true)
	}
	content := spacedPair(heading, cancel, inner) + "\n" + modeProgressMarker
	return box(content, width, 4, border, true)
}

func animateModeProgress(view string) string {
	if !strings.Contains(view, modeProgressMarker) {
		return view
	}
	filled := int(time.Now().UnixMilli() / 250 % 25)
	bar := text(strings.Repeat("━", filled), gold) + text(strings.Repeat("─", 24-filled), border)
	return strings.ReplaceAll(view, modeProgressMarker, bar)
}
