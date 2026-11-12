package main

import (
	"fmt"
	"os"

	"strings"
)

func mockupStart() string {
	color := activeInterface.Colors["mockup"]
	var r, g, b int
	fmt.Sscanf(color, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("\x1b[38;2;232;229;223;48;2;%d;%d;%dm", r, g, b)
}

func mockupPaint(body string) string {
	rows := strings.Split(body, "\n")
	for i, row := range rows {
		row = strings.ReplaceAll(row, "\x1b[0m", "\x1b[0m"+mockupStart())
		rows[i] = mockupStart() + row + "\x1b[0m"
	}
	return strings.Join(rows, "\n")
}
func mockupSurface(body string, width, height int) string {
	return mockupPaint(fit(body, width, height))
}
func mockupTag(label string) string { return mockupPaint(label) }
func scrollDiagnostic(top, end, total int) string {
	if os.Getenv("SOLOMON_TUI_SCROLL_TRACE") != "1" {
		return ""
	}
	return fmt.Sprintf("\x1b]777;scroll;%d;%d;%d\x1b\\", top+1, end, total)
}
func mockupCommand(name string) bool {
	return name == "/demo" || name == "/artifacts" || name == "/image"
}
