package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type diagramExample struct {
	name   string
	visual string
}

var diagramExamples = []diagramExample{
	{"Flowchart", "┌─────────┐      ┌────⌃────┐        ┌─────────┐\n│ Request │ ────▶‹ Valid?  ›─ Yes ─▶│ Execute │\n└─────────┘      └────⌄────┘        └─────────┘\n                      │ No\n                      ▼\n                  [Reject]"},
	{"Sequence", "User             Server           Database\n │── Sign in ─────▶│                  │\n │                 │── Verify ───────▶│\n │                 │◀──── OK ─────────│\n │◀── Session ─────│                  │"},
	{"Class", "            ┌─────────────────┐\n            │ Animal          │\n            ├─────────────────┤\n            │ name            │\n            │ speak()         │\n            └────────▲────────┘\n                     │\n       ┌─────────────┴─────────────┐\n       │                           │\n┌──────┴──────┐             ┌──────┴──────┐\n│ Cat         │             │ Dog         │\n├─────────────┤             ├─────────────┤\n│ meow()      │             │ bark()      │\n└─────────────┘             └─────────────┘"},
	{"State", "    ┌──────┐             ┌────────┐\n●──▶│ Idle │ ── Start ──▶│ Active │\n    └──┬───┘             └───┬────┘\n       ▲                     │ Failure\n       │ Reset               ▼\n       │                 ┌───────┐\n       └─────────────────│ Error │\n                         └───────┘"},
	{"Entity Relationship", "┌──────────┐          ┌─────────┐          ┌──────┐\n│ Customer │ ║─────○< │ Order   │ ║─────|< │ Item │\n└──────────┘  places  └─────────┘ contains └──────┘"},
	{"Use case", "   O          ╭──────────────────────╮\n  /|\\ ───────▶│       Purchase       │\n  / \\         ╰──────────────────────╯\nCustomer      ╭──────────────────────╮\n   └─────────▶│         Pay          │\n              ╰──────────────────────╯"},
	{"Gantt", "              Day 1 2 3 4 5 6 7 8\nAnalysis          ████\nDevelopment           ████████\nTesting                       ████"},
	{"User journey", "Search ───▶ Choose ───▶ Pay ───▶ Receive\n  ☺ 5         ☺ 4       ☹ 2        ☺ 5"},
	{"Git graph", "main     ●────┬───────────┬────●\n              │           │\nfeature       ╰──●────●───╯"},
	{"Pie", ""},
	{"Quadrant", "                  Value\n                    ▲\n   Do now           │       Plan\n          ● Bug     │\n                    │       ● Refactor\n ───────────────────┼──────────────────▶ Effort\n      ● Polish      │\n   Defer            │       Avoid"},
	{"XY", "Sales\n 80 │             █\n 60 │     █       █\n 40 │     █   █   █\n 20 │ █   █   █   █\n  0 └────────────────\n     Jan Feb Mar Apr\n     30  55  45  80"},
	{"Mindmap", "Project\n├── Frontend\n│   ├── GUI\n│   └── TUI\n└── Backend\n    ├── API\n    └── Database"},
	{"Timeline", "2024 ────────── 2025 ────────── 2026\n  │               │               ├── GUI\n  Prototype       Release         └── TUI"},
	{"Sankey", ""},
	{"Requirement", "┌────────────────────────┐\n│ «requirement» R1       │\n│ Authenticated access   │\n│ Risk: high             │\n└───────────▲────────────┘\n            ┆ verifies\n     ┌──────┴──────┐\n     │ Login test  │\n     └─────────────┘"},
	{"C4", "╭─────────╮      ╭───────────────╮      ╭─────────────╮\n│  User   │      │    Solomon    │      │ LLM provider│\n│ Person  │─────▶│ Local system  │─────▶│ Ext. system │\n╰─────────╯      ╰───────────────╯      ╰─────────────╯\n           uses                   calls"},
	{"Block", "┌─────────┐   ┌─────────┐   ┌─────────┐\n│ Input   │──▶│ Process │──▶│ Output  │\n└─────────┘   └─────────┘   └─────────┘"},
	{"Architecture", "╭─────────────────── Cloud ────────────────────╮\n│   ┌─────────┐            ┌──────────────┐    │\n│   │ API     │───────────▶│ Database     │    │\n│   └─────────┘            └──────────────┘    │\n╰──────────────────────────────────────────────╯"},
	{"Kanban", "┌─────────────┬─────────────┬─────────────┐\n│ To do       │ In progress │ Done        │\n├─────────────┼─────────────┼─────────────┤\n│ Tests       │ TUI renderer│ Prototype   │\n│ Docs        │             │             │\n└─────────────┴─────────────┴─────────────┘"},
	{"Packet", "Bit 0       4       8              16             31\n    ┌───────┬───────┬──────────────┬──────────────┐\n    │Version│ Type  │ Length       │ Data         │\n    └───────┴───────┴──────────────┴──────────────┘"},
	{"Radar", ""},
	{"Treemap", "┌─────────────────────────┬────────────────┐\n│ Frontend                │ Backend        │\n│ ┌─────────────┬───────┐ │ ┌──────┬─────┐ │\n│ │ GUI 40      │TUI 25 │ │ │API 20│DB 15│ │\n│ │             │       │ │ │      │     │ │\n│ └─────────────┴───────┘ │ └──────┴─────┘ │\n└─────────────────────────┴────────────────┘"},
	{"ZenUML", "User             Server           Database\n │── login() ─────▶│                  │\n │                 │── verify() ─────▶│\n │                 │◀── result ───────│\n │◀── session ─────│                  │"},
	{"Agent flow (flowchart)", "              ┌─────────────────┐\n              │   Coordinator   │\n              └────────┬────────┘\n        ┌──────────────┴───────────────┐\n        ▼                              ▼\n ┌──────┴───────┐             ┌────────┴────────┐\n │   Research   │             │   Development   │\n └──────┬───────┘             └────────┬────────┘\n        └──────────────┬───────────────┘\n                       ▼\n              ┌─────────────────┐\n              │    Verifier     │\n              └────────┬────────┘\n                       ▼\n              ┌─────────────────┐\n              │     Answer      │\n              └─────────────────┘"},
}

func mermaidGallery(width int, expanded bool) string {
	inner := max(1, width-5)
	cards := []string{text("Mermaid · diagram gallery", muted)}
	cards = append(cards, strings.Join(highlightedWrappedLine(text("Terminal layout examples · not a general Mermaid parser", muted), width), "\n"))
	for index, example := range diagramExamples {
		header := bold(fmt.Sprintf("%02d · %s", index+1, example.name), cyan)
		content := []string{header}
		if expanded {
			visual := example.visual
			switch example.name {
			case "Pie", "Sankey", "Radar":
				visual = mermaidImage(strings.ToLower(example.name), inner)
			}
			content = append(content, centerDiagram(visual, inner)...)
		}
		cards = append(cards, insetToolCard(content, inner, ""))
	}
	return strings.Join(cards, "\n\n")
}

func centerDiagram(visual string, width int) []string {
	rows := strings.Split(visual, "\n")
	maxWidth := 0
	for _, row := range rows {
		maxWidth = max(maxWidth, ansi.StringWidth(row))
	}
	padding := strings.Repeat(" ", max(0, (width-maxWidth)/2))
	for i := range rows {
		rows[i] = padding + rows[i]
	}
	return rows
}
