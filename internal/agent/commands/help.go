package commands

import (
	"fmt"
	"io"
	"sort"
)

func Registry() [][]string {
	return [][]string{
		{"/plan", "planning tools only"},
		{"/build", "build tools (shell, files, subagent)"},
		{"/clear", "clear terminal (ANSI)"},
		{"/connect", "add OpenAI-compatible provider"},
		{"/exit, /quit", "exit and show how to resume"},
		{"/help", "this list"},
		{"/language", "/language | /language <language> | /language clear — reply language (default English; saved; system prompt)"},
		{"/log", "/log {error|warning|info|debug|result} visible log verbosity"},
		{"/max_response", "/max_response | /max_response <n> assistant output cap (tokens, n>=1)"},
		{"/models", "list models and switch current model"},
		{"/reasoning", "/reasoning | /reasoning {none|low|med|high} main chat reasoning_effort"},
		{"/resume", "/resume | /resume last | /resume <id|title>"},
		{"/stats", "toggle token usage line after assistant turns (saved)"},
		{"/summarize, /compact", "summarize full chat; summary + last 8 msgs; then /clear"},
		{"/thinking", "/thinking toggles preview; /thinking on|off streamed model reasoning (light yellow)"},
		{"/timeout", "/timeout <minutes> subagent segment (1–180)"},
	}
}

func WriteHelp(w io.Writer) {
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
}
