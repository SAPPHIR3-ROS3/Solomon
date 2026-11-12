package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/interaction"
	tea "github.com/charmbracelet/bubbletea"
)

func clickZone(id, body string) string { return interaction.Mark(id, body) }
func (m model) mouseClick(event tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.liveCfg.Mouse || event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft || event.Shift {
		return m, nil
	}
	wasWorkedForActive := m.workedForActive()
	wasNotificationActive := m.notificationActive()
	id := m.zones.Hit(event.X, event.Y)
	kind, value, _ := strings.Cut(id, ":")
	switch kind {
	case "layout":
		variant, err := strconv.Atoi(value)
		if err != nil || (variant != 2 && variant != 4) {
			return m, nil
		}
		anchor, offset := m.scrollAnchor()
		atBottom := m.scroll == 0
		m.variant = variant
		m.palette = false
		m.componentScroll = 0
		if !atBottom {
			m = m.restoreAnchor(anchor, offset)
		}
	case "action":
		switch value {
		case "components":
			m.componentOpen = !m.componentOpen
		case "quit":
			return m, tea.Quit
		case "cancel_mode_preview":
			m.componentState = 1
			m.foldVersion++
		case "close_subchat_preview":
			delete(m.sections, "preview.subagent.open")
			m.foldVersion++
		case "toggle_reasoning":
			if m.componentState == 1 {
				m.componentState = 0
			} else {
				m.componentState = 1
			}
		case "toggle_subagent_activity":
			if m.componentState == 1 {
				m.componentState = 2
			} else {
				m.componentState = 1
			}
		case "component_bottom":
			_, rows := m.componentBody(m.width-min(24, m.width/3), m.height-3)
			m.componentScroll = max(0, len(rows)-m.height+3)
		case "scroll_bottom":
			m.scroll = 0
		case "new_thread":
			m = m.newThread()
		case "toggle_details":
			anchor, offset := m.scrollAnchor()
			m.details = !m.details
			m = m.restoreAnchor(anchor, offset)
		}
	case "toolbar":
		switch value {
		case "model":
			if m.guiMenu == "model" {
				m.guiMenu = ""
			} else {
				m.guiMenu = "model"
			}
			m.menuHidden = true
		case "reasoning":
			if m.guiMenu == "reasoning" {
				m.guiMenu = ""
			} else {
				m.guiMenu = "reasoning"
			}
			m.menuHidden = true
		case "mode":
			m.selected = (m.selected + 1) % 2
		case "send":
			if len(m.streaming) > 0 {
				m.streaming = nil
				m.revealed = 0
				m.status = "Interrupted"
				return m, nil
			}
			return m.submitPrompt()
		}
	case "picker":
		index, err := strconv.Atoi(value)
		if err != nil {
			return m, nil
		}
		if m.guiMenu == "model" && index >= 0 && index < len(previewModels) {
			m.modelChoice = index
			m.guiMenu = ""
		}
		if m.guiMenu == "reasoning" && index >= 0 && index < len(previewReasoning) {
			m.reasoningChoice = index
			m.guiMenu = ""
		}
	case "slash":
		matches := m.slashMatches()
		index, err := strconv.Atoi(value)
		if err == nil && index >= 0 && index < len(matches) {
			returnCommand := matches[index].name
			m, _ = m.runSlashCommand(returnCommand)
		}
	case "component":
		index, err := strconv.Atoi(value)
		if err == nil && index >= 0 && index < len(componentNames) {
			m.componentIndex = index
			m.componentState %= m.componentStateCount()
			if (componentNames[index] == "Worked for" || componentNames[index] == "Reasoning") && m.componentState == 2 {
				resetWorkedFor()
			}
			m.componentScroll = 0
		}
	case "notification_copy":
		return m, m.copyNotification(value)
	case "subchat":
		if value != "0" && value != "1" {
			return m, nil
		}
		if m.sections == nil {
			m.sections = map[string]bool{}
		}
		m.sections["preview.subagent.1"] = value == "1"
		m.sections["preview.subagent.open"] = true
		m.subchatScroll = 0
		m.componentScroll = 0
		m.foldVersion++
	case "component_state":
		if (componentNames[m.componentIndex%len(componentNames)] == "Worked for" || componentNames[m.componentIndex%len(componentNames)] == "Reasoning") && m.componentState == 1 {
			resetWorkedFor()
		}
		if componentNames[m.componentIndex%len(componentNames)] != "Rate limit" && componentNames[m.componentIndex%len(componentNames)] != "Interrupted generation" {
			m.componentState = (m.componentState + 1) % m.componentStateCount()
		}
		m.componentScroll = 0
	case "files":
		if value == "header" {
			m.componentState = (m.componentState + 1) % 3
			m.foldVersion++
		}
	case "section":
		anchor, offset := m.scrollAnchor()
		if m.sections == nil {
			m.sections = map[string]bool{}
		}
		m.sections[value] = !m.sections[value]
		m.foldVersion++
		if !m.componentOpen {
			m = m.restoreAnchor(anchor, offset)
		}
	case "artifact":
		if m.componentOpen {
			index, err := strconv.Atoi(value)
			if err != nil || index < 0 {
				return m, nil
			}
			if m.sections == nil {
				m.sections = map[string]bool{}
			}
			key := fmt.Sprintf("component-%d.artifact.%d", m.componentIndex, index)
			m.sections[key] = !m.sections[key]
			m.foldVersion++
			return m, nil
		}
		index, err := strconv.Atoi(value)
		if err == nil && index >= 0 && index < len(m.messages) {
			anchor, offset := m.scrollAnchor()
			if m.folded == nil {
				m.folded = map[int]bool{}
			}
			m.folded[index] = !m.folded[index]
			m.foldVersion++
			m = m.restoreAnchor(anchor, offset)
		}
	}
	if m.notificationActive() && !wasNotificationActive {
		notificationStart = time.Now()
		return m, pulse()
	}
	if m.workedForActive() && !wasWorkedForActive {
		return m, pulse()
	}
	return m, nil
}
