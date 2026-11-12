package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	catalog "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var componentNames = func() []string {
	names := []string{}
	for _, component := range catalog.Default() {
		names = append(names, component.Label)
	}
	return names
}()

func (m model) componentStateCount() int {
	if componentNames[m.componentIndex%len(componentNames)] == "Tool call" {
		return 4
	}
	return 3
}

func (m model) handleComponentKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, rows := m.componentBody(m.width-min(24, m.width/3), m.height-3)
	limit := max(0, len(rows)-m.height+3)
	m.componentScroll = min(max(0, m.componentScroll), limit)
	if (m.sections["preview.subagent.open"] && componentNames[m.componentIndex%len(componentNames)] == "Subagent activity") || componentNames[m.componentIndex%len(componentNames)] == "Subagent chat" {
		width := m.width - min(24, m.width/3)
		if key.String() == "f17" {
			m.subchatScroll = max(0, m.subchatScroll-3)
			return m, nil
		}
		if key.String() == "f18" {
			m.subchatScroll = min(m.subchatScrollLimit(width), m.subchatScroll+3)
			return m, nil
		}
	}
	wasWorkedForActive := m.workedForActive()
	wasNotificationActive := m.notificationActive()
	action := m.liveCfg.Action("components", key.String())
	switch action {
	case "previous":
		m.componentIndex = (m.componentIndex + len(componentNames) - 1) % len(componentNames)
		m.componentState %= m.componentStateCount()
		m.componentScroll = 0
	case "next":
		m.componentIndex = (m.componentIndex + 1) % len(componentNames)
		m.componentState %= m.componentStateCount()
		m.componentScroll = 0
	case "state":
		if componentNames[m.componentIndex%len(componentNames)] != "Rate limit" && componentNames[m.componentIndex%len(componentNames)] != "Interrupted generation" {
			m.componentState = (m.componentState + 1) % m.componentStateCount()
		}
		m.componentScroll = 0
	case "dismiss", "components":
		m.componentOpen = false
	case "layout_2", "layout_4":
		m.variant, _ = strconv.Atoi(strings.TrimPrefix(action, "layout_"))
		m.componentScroll = 0
	case "scroll_start":
		m.componentScroll = 0
	case "scroll_end":
		m.componentScroll = limit
	case "page_up":
		m.componentScroll = max(0, m.componentScroll-max(1, m.height/2))
	case "page_down":
		m.componentScroll = min(limit, m.componentScroll+max(1, m.height/2))
	}
	if (componentNames[m.componentIndex%len(componentNames)] == "Worked for" || componentNames[m.componentIndex%len(componentNames)] == "Reasoning") && m.componentState == 2 && (action == "state" || action == "next" || action == "previous") {
		resetWorkedFor()
	}
	if key.String() == "f17" {
		m.componentScroll = max(0, m.componentScroll-3)
	}
	if key.String() == "f18" {
		m.componentScroll = min(limit, m.componentScroll+3)
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
func (m model) componentMap() string {
	if m.width < 42 || m.height < 24 {
		return "Resize to at least 42 × 24"
	}
	sidebarWidth := min(24, m.width/3)
	panelWidth := m.width - sidebarWidth
	height := m.height - 2
	selected := m.componentIndex % len(componentNames)
	menu := []string{bold("COMPONENT MAP", gold), ""}
	menuCount := max(1, height-5)
	menuStart := max(0, selected-menuCount+1)
	menuEnd := min(len(componentNames), menuStart+menuCount)
	for i := menuStart; i < menuEnd; i++ {
		name := componentNames[i]
		marker := "  "
		if i == selected {
			marker = "› "
		}
		menu = append(menu, clickZone(fmt.Sprintf("component:%d", i), text(marker+name, ink)))
	}
	menu = append(menu, "", text("↑↓ component", muted), text(keyCaption(configuredHint("components", "state"))+" state · "+keyCaption(configuredHint("components", "dismiss"))+" back", muted))
	sidebar := mockupSurface(strings.Join(menu, "\n"), sidebarWidth, height)
	body, rows := m.componentBody(panelWidth, height-1)
	top := min(max(0, m.componentScroll), max(0, len(rows)-height+1))
	visible := append([]string{}, rows[top:min(len(rows), top+height-1)]...)
	if top+height-1 < len(rows) && len(visible) >= 3 {
		m.overlayBottomButton(visible, "action:component_bottom", panelWidth)
	}
	body = fit(strings.Join(visible, "\n"), panelWidth, height-1)
	titleText := componentNames[selected]
	if titleText != "Rate limit" && titleText != "Interrupted generation" {
		titleText = clickZone("component_state:next", fmt.Sprintf("%s · %s", titleText, []string{"expanded", "collapsed", "active", "failed"}[m.componentState%m.componentStateCount()]))
	}
	title := mockupSurface(titleText, panelWidth, 1)
	panel := title + "\n" + body
	header := mockupSurface("Solomon · live component map", m.width, 1)
	return fit(header+"\n"+lipgloss.JoinHorizontal(lipgloss.Top, sidebar, panel)+"\n"+m.footer(), m.width, m.height)
}
func (m model) renderComponent(width, height int) string {
	sample := m
	sample.streaming = nil
	sample.revealed = 0
	sample.folded = nil
	sample.foldVersion = 0
	sample.cache = &transcriptCache{}
	sample.details = m.componentState != 1
	sample.sectionPrefix = fmt.Sprintf("component-%d.", m.componentIndex)
	sample.componentOpen = false
	name := componentNames[m.componentIndex%len(componentNames)]
	if body, ok := m.supplementaryComponent(name, width); ok {
		return body
	}
	switch name {
	case "Welcome":
		sample.messages = nil
		return sample.welcome(width, height)
	case "Composer":
		if m.componentState == 2 {
			sample.draft = []rune("Review the editor\nand preserve binary files")
		}
		return sample.composerDock(width) + "\n" + sample.metadata(width)
	case "Slash commands":
		sample.draft = []rune("/")
		return sample.composerDock(width)
	case "searchTools":
		sample.messages = presentationMessages("search")
	case "Tool call":
		sample.messages = presentationMessages("tool_call")
		if (m.componentState == 2 || m.componentState == 3) && len(sample.messages) > 0 {
			var tool recordedTool
			if json.Unmarshal([]byte(sample.messages[0].text), &tool) == nil {
				if m.componentState == 2 {
					tool.Status = "running"
					tool.Result = ""
				} else {
					tool.Status = "error"
					tool.Result = `{"error":"Tool execution failed: unable to read the requested file.","tool_calls":1,"duration_ms":240}`
				}
				data, err := json.Marshal(tool)
				if err == nil {
					sample.messages[0].text = string(data)
				}
			}
		}
	case "orchestrate":
		sample.messages = presentationMessages("code")
	case "User message":
		sample.messages = presentationMessages("user")
	case "Image":
		return imagePreview(width)
	case "Transcript":
		sample.messages = sampleMessages()
	default:
		role := map[string]string{"Tool result": "OUTPUT", "Markdown": "MARKDOWN", "Subagent": "SUBAGENT", "Checkpoint": "CHECKPOINT", "Notification": "NOTICE"}[name]
		for _, entry := range galleryMessages() {
			if entry.role == role {
				sample.messages = []message{entry}
				break
			}
		}
	}
	if m.componentState == 2 && name != "Tool call" {
		sample.streaming = []rune("Live state: this component uses the same rendering path as the conversation.")
		sample.revealed = len(sample.streaming)
	}
	sample.folded = map[int]bool{}
	for index := range sample.messages {
		key := fmt.Sprintf("component-%d.artifact.%d", m.componentIndex, index)
		sample.folded[index] = m.sections[key]
	}
	rows, _ := sample.transcriptLines(width)
	return strings.Join(rows, "\n")
}
