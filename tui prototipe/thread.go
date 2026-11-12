package main

func (m model) newThread() model {
	m.messages = nil
	m.folded = nil
	m.foldVersion++
	m.streaming = nil
	m.revealed = 0
	m.draft = nil
	m.scroll = 0
	m.palette = false
	m.guiMenu = ""
	m.suggestion = 0
	m.menuHidden = false
	m.historyIndex = -1
	m.status = "New thread"
	return m
}

func (m model) loadMessages(messages []message) model {
	m.messages = messages
	m.folded = nil
	m.foldVersion++
	m.streaming = nil
	m.revealed = 0
	return m.scrollToTop()
}
