package main

import (
	"encoding/json"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/live"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/state"
)

type sessionMessage struct {
	Role string
	Text string
}
type sessionSnapshot struct {
	Variant, Width, Height, Scroll, Selected, Frame int
	Draft                                           string
	Messages                                        []sessionMessage
	Streaming                                       string
	Revealed                                        int
	Details, Palette                                bool
	History                                         []string
	HistoryIndex, Suggestion                        int
	MenuHidden                                      bool
	Folded                                          map[int]bool
	Sections                                        map[string]bool
	ModelChoice, ReasoningChoice                    int
	GUIMenu                                         string
	ComponentOpen                                   bool
	ComponentIndex, ComponentState, ComponentScroll int
	SubchatScroll                                   int
	Config                                          live.Config
}

func (m model) saveSession(token string) error {
	value := sessionSnapshot{Variant: m.variant, Width: m.width, Height: m.height, Scroll: m.scroll, Selected: m.selected, Frame: m.frame, Draft: string(m.draft), Streaming: string(m.streaming), Revealed: m.revealed, Details: m.details, Palette: m.palette, History: m.history, HistoryIndex: m.historyIndex, Suggestion: m.suggestion, MenuHidden: m.menuHidden, Folded: m.folded, Sections: m.sections, ModelChoice: m.modelChoice, ReasoningChoice: m.reasoningChoice, GUIMenu: m.guiMenu, ComponentOpen: m.componentOpen, ComponentIndex: m.componentIndex, ComponentState: m.componentState, ComponentScroll: m.componentScroll, SubchatScroll: m.subchatScroll, Config: m.liveCfg}
	for _, entry := range m.messages {
		value.Messages = append(value.Messages, sessionMessage{entry.role, entry.text})
	}
	return state.Save(os.Getenv("SOLOMON_TUI_STATE_PATH"), token, value)
}
func (m *model) restoreSession() error {
	path := os.Getenv("SOLOMON_TUI_STATE_PATH")
	if path == "" {
		return nil
	}
	var value sessionSnapshot
	_, err := state.Load(path, &value)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if value.Variant < 1 || value.Variant > 4 || value.Selected < 0 || value.Selected > 1 {
		return os.ErrInvalid
	}
	savedConfig, err := json.Marshal(value.Config)
	if err != nil {
		return err
	}
	value.Config, err = live.Decode(savedConfig)
	if err != nil {
		return err
	}
	m.variant = value.Variant
	if m.variant != 2 && m.variant != 4 {
		m.variant = 4
	}
	m.width = value.Width
	m.height = value.Height
	m.scroll = max(0, value.Scroll)
	m.selected = value.Selected
	m.frame = value.Frame
	m.draft = []rune(value.Draft)
	m.messages = nil
	for _, entry := range value.Messages {
		m.messages = append(m.messages, message{entry.Role, entry.Text})
	}
	m.streaming = []rune(value.Streaming)
	m.revealed = min(max(0, value.Revealed), len(m.streaming))
	m.details = value.Details
	m.palette = value.Palette
	m.history = value.History
	m.historyIndex = value.HistoryIndex
	m.suggestion = value.Suggestion
	m.menuHidden = value.MenuHidden
	m.folded = value.Folded
	m.sections = value.Sections
	m.modelChoice = max(0, value.ModelChoice)
	m.reasoningChoice = max(0, value.ReasoningChoice)
	m.guiMenu = value.GUIMenu
	m.componentOpen = value.ComponentOpen
	m.componentIndex = max(0, value.ComponentIndex)
	m.componentState = max(0, value.ComponentState)
	m.componentScroll = max(0, value.ComponentScroll)
	m.subchatScroll = max(0, value.SubchatScroll)
	if m.liveError != "" {
		m.liveCfg = value.Config
		m.watcher.Current = value.Config
		applyInterface(value.Config)
	}
	m.sessionRestored = true
	return nil
}

func currentReloadRequest(token string) bool {
	data, err := os.ReadFile(os.Getenv("SOLOMON_TUI_RELOAD_REQUEST"))
	if err != nil {
		return false
	}
	var request struct{ Token string }
	return json.Unmarshal(data, &request) == nil && request.Token == token
}
