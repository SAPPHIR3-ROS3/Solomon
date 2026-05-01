package commands

import (
	"fmt"
	"io"
	"sort"

	"solomon/internal/skills"
)

func Registry() [][]string {
	return [][]string{
		{"/plan", "planning tools only"},
		{"/add", "/add npx skills add ... | https://skills.sh/... | skill <path/to/.md> [name] [global|project|local], default global"},
		{"/remove skill", "/remove skill <name>"},
		{"/skills", "/skills — list installed skills (Local → Project → Global; empty sections omitted)"},
		{"/build", "build tools (shell, files, subagent)"},
		{"/clear", "clear terminal (ANSI)"},
		{"/connect", "add provider (checks /models), pick model (0 current, 1-20 listed, truncated: 21=id, paste id)"},
		{"/exec", "/exec <prompt> | /exec \"prompt with spaces\" — send one user message"},
		{"/exit, /quit", "exit and show how to resume"},
		{"/help", "this list"},
		{"/language", "/language | /language <language> | /language clear — reply language (default English; saved; system prompt)"},
		{"/legacytools", "/legacytools | /legacy | /legacytools on|off — parse Tool: lines from assistant text + inject syntax into system prompt"},
		{"/log", "/log {error|warning|info|debug|result} visible log verbosity"},
		{"/max_response", "/max_response | /max_response <n> assistant output cap (tokens, n>=1)"},
		{"/models", "list models and switch current model"},
		{"/new", "start a new chat session (empty transcript; prior chat stays saved on disk)"},
		{"/reasoning", "/reasoning | /reasoning {none|low|med|high} main chat; subagent always none"},
		{"/resume", "/resume | /resume last | /resume <id|title>"},
		{"/stats", "toggle token usage line after assistant turns (saved)"},
		{"/summarize, /compact", "summarize full chat; summary + last 8 msgs; then /clear"},
		{"/threshold", "/threshold | /threshold <n> auto /summarize when prompt_tokens >= n (n>=32768; default 131072; needs API usage)"},
		{"/thinking", "/thinking toggles preview; /thinking on|off streamed reasoning (dim gray); tool echoes (yellow)"},
		{"/timeout", "/timeout <minutes> subagent segment (1–180)"},
	}
}

func WriteHelp(w io.Writer, projHex, projRoot string) {
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
	_ = skills.WriteSkillsHelpSection(w, maxCmd, projHex, projRoot)
}
