package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m model) runSlashCommand(input string) (model, bool) {
	command := strings.TrimSpace(input)
	if !strings.HasPrefix(command, "/") {
		return m, false
	}
	switch command {
	case "/components":
		m.componentOpen = true
	case "/clear", "/new":
		m = m.newThread()
	case "/demo":
		m = m.loadMessages(sampleMessages())
	case "/artifacts":
		m = m.loadMessages(galleryMessages())
	case "/image":
		m = m.loadMessages(imageMessages())
	case "/agent":
		m.selected = 0
	case "/chat":
		m.selected = 1
	case "/help":
		m.messages = append(m.messages, message{"NOTICE", "Keyboard shortcuts\nF1–F4 layouts · F5 archive · F6 gallery · Ctrl+N new thread\nPgUp/PgDown and mouse wheel scroll · Alt+Up/Down jump artifacts\nCtrl+O toggle one card · Ctrl+D toggle all cards\nCtrl+P model · Ctrl+R reasoning · / shows commands\nCtrl+J or Alt+Enter adds a newline · Ctrl+Q exits"})
	default:
		m.messages = append(m.messages, message{"NOTICE", "Unknown command\n" + command + "\nType / to see the supported commands."})
	}
	m.history = append(m.history, command)
	m.historyIndex = -1
	m.draft = nil
	m.suggestion = 0
	m.menuHidden = false
	return m, true
}

func (m model) submitPrompt() (tea.Model, tea.Cmd) {

	if m.palette {
		m.variant = m.variant%4 + 1
		m.palette = false
		return m, nil
	}
	prompt := strings.TrimSpace(string(m.draft))
	if updated, handled := m.runSlashCommand(prompt); handled {
		return updated, nil
	}
	if prompt != "" && len(m.streaming) == 0 {
		m.history = append(m.history, prompt)
		m.historyIndex = -1
		m.draft = nil
		m.scroll = 0
		m.messages = append(m.messages, message{"YOU", prompt})
		m.streaming = []rune("This is a simulated response, not a live agent run. Your message was received locally. Try resizing the terminal, scrolling the conversation, or switching layouts with F1–F4. Press Ctrl+N to start a new thread and see the banner again. The composer and conversation stay with you across all four designs.")
		m.status = "Generating response…"
		return m, pulse()
	}

	return m, nil
}
