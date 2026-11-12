package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/interaction"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/live"
	tea "github.com/charmbracelet/bubbletea"
)

var defaultVariant = "4"

type tickMsg time.Time

type message struct{ role, text string }

type model struct {
	variant, width, height, scroll, frame, selected int
	draft                                           []rune
	messages                                        []message
	streaming                                       []rune
	revealed                                        int
	details, palette                                bool
	status                                          string
	history                                         []string
	historyIndex                                    int
	suggestion                                      int
	menuHidden                                      bool
	cache                                           *transcriptCache
	folded                                          map[int]bool
	foldVersion                                     int
	sections                                        map[string]bool
	sectionPrefix                                   string
	modelChoice                                     int
	reasoningChoice                                 int
	guiMenu                                         string
	liveCfg                                         live.Config
	watcher                                         *live.Watcher
	configPath                                      string
	liveError                                       string
	devStatus                                       string
	handoffToken                                    string
	componentOpen                                   bool
	componentIndex                                  int
	componentState                                  int
	componentScroll                                 int
	subchatScroll                                   int
	sessionRestored                                 bool
	zones                                           *interaction.Map
	mouseEvents                                     *mouseQueue
	componentCache                                  *componentRenderCache
}

func newModel(variant int) model {
	cfg := live.Default()
	path, pathErr := live.Path()
	watcher := live.NewWatcher(path)
	_, err := watcher.Poll()
	if err == nil {
		cfg = watcher.Current
	}
	if pathErr != nil {
		err = pathErr
	}
	applyInterface(cfg)
	return model{variant: variant, width: 100, height: 32, details: true, historyIndex: -1, status: "Ready", cache: &transcriptCache{}, componentCache: &componentRenderCache{}, zones: &interaction.Map{}, mouseEvents: &mouseQueue{}, liveCfg: cfg, watcher: watcher, configPath: path, liveError: cleanNotice(err)}
}

func pulse() tea.Cmd {
	return tea.Tick(35*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m model) Init() tea.Cmd {
	if len(m.streaming) > 0 || m.workedForActive() || m.notificationActive() || m.notificationActive() {
		return tea.Batch(m.pollInterface(), pulse())
	}
	return m.pollInterface()
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case interfaceMsg:
		index, offset := m.scrollAnchor()
		atBottom := m.scroll == 0
		if v.err != nil {
			m.liveError = cleanNotice(v.err)
		}
		var mouseCommand tea.Cmd
		if v.changed {
			if m.liveCfg.Mouse != v.cfg.Mouse {
				if v.cfg.Mouse {
					mouseCommand = tea.EnableMouseCellMotion
				} else {
					mouseCommand = tea.DisableMouse
				}
			}
			m.liveCfg = v.cfg
			applyInterface(v.cfg)
			m.liveError = ""
			m.cache.valid = false
			if !atBottom {
				m = m.restoreAnchor(index, offset)
			}
		}
		m.devStatus = v.devStatus
		if v.request != "" && v.request != m.handoffToken {
			if err := m.saveSession(v.request); err != nil {
				m.liveError = cleanNotice(err)
			} else {
				if currentReloadRequest(v.request) {
					m.handoffToken = v.request
					return m, tea.Quit
				}
			}
		}
		return m, tea.Batch(m.pollInterface(), mouseCommand)
	case notificationCopyResult:
		if v.err != nil {
			m.status = "Clipboard write failed: " + v.err.Error()
		}
	case tea.WindowSizeMsg:
		index, offset := m.scrollAnchor()
		atBottom := m.scroll == 0
		m.width, m.height = v.Width, v.Height
		if !atBottom {
			m = m.restoreAnchor(index, offset)
		}
	case tickMsg:
		index, offset := m.scrollAnchor()
		pinned := m.scroll > 0
		m.frame++
		if m.revealed < len(m.streaming) {
			m.revealed = min(len(m.streaming), m.revealed+3)
			if m.revealed == len(m.streaming) {
				m.messages = append(m.messages, message{"SOLOMON", string(m.streaming)})
				m.streaming = nil
				m.revealed = 0
				m.status = "Ready for your next message"
			}
		}
		if pinned {
			m = m.restoreAnchor(index, offset)
		}
		if len(m.streaming) > 0 || m.workedForActive() {
			return m, pulse()
		}
		return m, nil
	case tea.MouseMsg:
		if !m.liveCfg.Mouse {
			return m, nil
		}
		if !tea.MouseEvent(v).IsWheel() {
			return m.mouseClick(v)
		}
		if m.componentOpen {
			key := "f18"
			if v.Button == tea.MouseButtonWheelUp {
				key = "f17"
			}
			return m.handleComponentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
		switch v.Button {
		case tea.MouseButtonWheelUp:
			m = m.moveScroll(3)
		case tea.MouseButtonWheelDown:
			m = m.moveScroll(-3)
		}
	case tea.KeyMsg:
		if v.String() == "ctrl+@" {
			if event, ok := m.mouseEvents.Pop(); ok {
				return m.mouseClick(event)
			}
			return m, nil
		}
		if v.String() == "ctrl+c" || m.liveCfg.Action("global", v.String()) == "quit" {
			return m, tea.Quit
		}
		if m.componentOpen {
			return m.handleComponentKey(v)
		}
		var handled bool
		m, handled = m.guiControlKey(v)
		if handled {
			return m, nil
		}
		acceptedSuggestion := len(m.slashMatches()) > 0 && m.liveCfg.Action("suggestions", v.String()) == "accept"
		m, handled = m.slashKey(v)
		if handled {
			return m, nil
		}
		action := m.liveCfg.Action("composer", v.String())
		if acceptedSuggestion {
			action = "send"
		}
		if !m.liveCfg.Mouse && (v.String() == "f17" || v.String() == "f18") {
			return m, nil
		}
		if v.String() == "f17" {
			action = "scroll_up"
		}
		if v.String() == "f18" {
			action = "scroll_down"
		}
		switch action {
		case "quit":
			return m, tea.Quit
		case "layout_2", "layout_4":
			index, offset := m.scrollAnchor()
			atBottom := m.scroll == 0
			m.variant, _ = strconv.Atoi(strings.TrimPrefix(action, "layout_"))
			if !atBottom {
				m = m.restoreAnchor(index, offset)
			}
		case "archive":
			m = m.loadMessages(sampleMessages())
			m = m.scrollToTop()
			m.streaming = nil
			m.revealed = 0
		case "components":
			m.componentOpen = true
		case "gallery":
			m = m.loadMessages(galleryMessages())
			m = m.scrollToTop()
			m.streaming = nil
			m.revealed = 0
		case "scroll_start":
			m = m.scrollToTop()
		case "scroll_end":
			m.scroll = 0
		case "artifact_previous":
			m = m.jumpArtifact(-1)
		case "artifact_next":
			m = m.jumpArtifact(1)
		case "new_thread":
			m = m.newThread()
		case "toggle_artifact":
			m = m.toggleArtifact()
		case "toggle_details":
			index, offset := m.scrollAnchor()
			atBottom := m.scroll == 0
			m.details = !m.details
			if !atBottom {
				m = m.restoreAnchor(index, offset)
			}
		case "commands":
			m.palette = !m.palette
		case "cancel":
			if m.palette {
				m.palette = false
			} else if len(m.streaming) > 0 {
				m.streaming = nil
				m.revealed = 0
				m.status = "Interrupted"
			}
		case "scroll_up":
			m = m.moveScroll(3)
		case "scroll_down":
			m = m.moveScroll(-3)
		case "page_up":
			m = m.moveScroll(max(1, m.scrollView().height-1))
		case "page_down":
			m = m.moveScroll(-max(1, m.scrollView().height-1))
		case "toggle_mode":
			m.selected = (m.selected + 1) % 2
			m.status = []string{"Mode: Agent · code mode", "Mode: Chat"}[m.selected]
		case "history_previous":
			if len(m.history) > 0 {
				m.historyIndex = min(len(m.history)-1, m.historyIndex+1)
				m.draft = []rune(m.history[len(m.history)-1-m.historyIndex])
			}
		case "history_next":
			if m.historyIndex > 0 {
				m.historyIndex--
				m.draft = []rune(m.history[len(m.history)-1-m.historyIndex])
			} else {
				m.historyIndex = -1
				m.draft = nil
			}
		case "backspace":
			if len(m.draft) > 0 {
				m.draft = m.draft[:len(m.draft)-1]
			}
		case "clear_draft":
			m.draft = nil
		case "newline":
			m.draft = append(m.draft, '\n')
		case "send":
			return m.submitPrompt()
		default:
			if v.Type == tea.KeyRunes || v.Type == tea.KeySpace {
				m.draft = append(m.draft, v.Runes...)
			}
		}
	}
	return m, nil
}

func main() {
	filtered, err := interfaceArgs(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Args = filtered
	if len(os.Args) > 1 && os.Args[1] == "--init-config" {
		path, err := live.Path()
		if err == nil {
			err = live.Export(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(path)
		return
	}
	variant, _ := strconv.Atoi(defaultVariant)
	switch strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") {
	case "orbit":
		variant = 2
	case "mission", "laboratory":
		variant = 4
	}
	if len(os.Args) > 1 {
		if n, e := strconv.Atoi(os.Args[1]); e == nil && n >= 1 && n <= 4 {
			variant = n
		}
	}
	if variant != 2 && variant != 4 {
		variant = 4
	}
	if len(os.Args) > 1 && os.Args[1] == "--snapshot" {
		m := newModel(variant)
		if len(os.Args) > 3 {
			m.width, _ = strconv.Atoi(os.Args[2])
			m.height, _ = strconv.Atoi(os.Args[3])
		}
		if len(os.Args) > 4 {
			switch os.Args[4] {
			case "demo":
				m = m.loadMessages(sampleMessages())
				m = m.scrollToTop()
			case "wrapped_strings", "tool_call", "search", "code", "user":
				m.messages = presentationMessages(os.Args[4])
				m = m.scrollToTop()
			case "image":
				m = m.loadMessages(imageMessages())
				m = m.scrollToTop()
			case "model":
				m.guiMenu = "model"
			case "reasoning":
				m.guiMenu = "reasoning"
			case "slash":
				m.draft = []rune("/")
			case "gallery":
				m = m.loadMessages(galleryMessages())
				m = m.scrollToTop()
			case "palette":
				m.palette = true
			case "stream":
				m = m.loadMessages(sampleMessages())
				m.streaming = []rune("A simulated response in progress…")
				m.revealed = 15
			}
		}
		fmt.Print(snapshot(m))
		return
	}
	m := newModel(variant)
	if err := live.Export(m.configPath); err != nil {
		m.liveError = cleanNotice(err)
	}
	if err := m.restoreSession(); err != nil {
		fmt.Fprintln(os.Stderr, "TUI state restore:", err)
		os.Exit(1)
	}
	if !m.sessionRestored && len(os.Args) > 1 {
		switch os.Args[1] {
		case "--components":
			m.componentOpen = true
		case "--image":
			m = m.loadMessages(imageMessages())
			m = m.scrollToTop()
		case "--demo":
			m = m.loadMessages(sampleMessages())
			m = m.scrollToTop()
		case "--artifacts":
			m = m.loadMessages(galleryMessages())
			m = m.scrollToTop()
		case "--transcript":
			m = m.loadMessages(sampleMessages())
			if len(os.Args) > 2 && os.Args[2] == "gallery" {
				m = m.loadMessages(galleryMessages())
			}
			lines, _ := m.transcriptLines(100)
			fmt.Print(strings.Join(lines, "\n"))
			return
		}
	}
	if _, err := runPreview(m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
