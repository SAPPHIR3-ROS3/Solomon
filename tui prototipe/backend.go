package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"solomon-tui-prototypes/daemon"
)

type backendState struct {
	generation    uint64
	active        bool
	stopping      bool
	streamEnded   bool
	terminal      bool
	stopAccepted  bool
	client        *daemon.Client
	project, chat string
	cancel        context.CancelFunc
	events        chan backendMsg
}

type backendMsg struct {
	generation    uint64
	client        *daemon.Client
	project, chat string
	event         *daemon.Event
	err           error
	done, stopped bool
	attempted     bool
}

func waitBackend(ch <-chan backendMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m model) sendBackend(prompt string) (tea.Model, tea.Cmd) {
	m.backend.generation++
	generation := m.backend.generation
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan backendMsg, 32)
	client, project, chat := m.backend.client, m.backend.project, m.backend.chat
	m.backend.active, m.backend.stopping = true, false
	m.backend.streamEnded, m.backend.terminal, m.backend.stopAccepted = false, false, false
	m.backend.cancel, m.backend.events = cancel, ch
	m.history = append(m.history, prompt)
	m.historyIndex = -1
	m.draft = nil
	m.scroll = 0
	m.messages = append(m.messages, message{"YOU", prompt})
	m.status = "Connecting to daemon…"
	go func() {
		defer close(ch)
		emit := func(v backendMsg) bool {
			v.generation = generation
			select {
			case ch <- v:
				return true
			case <-ctx.Done():
				return false
			}
		}
		var err error
		if client == nil {
			client, err = daemon.Discover(ctx, "")
		}
		if err == nil && project == "" {
			var p daemon.Project
			p, err = client.CurrentProject(ctx, os.Getenv("SOLOMON_TUI_WORKSPACE"))
			project = p.ID
		}
		if err == nil && chat == "" {
			var c daemon.Chat
			c, err = client.CreateChat(ctx, project, "")
			chat = c.ID
		}
		attempted := false
		if err == nil {
			if !emit(backendMsg{client: client, project: project, chat: chat}) {
				return
			}
			attempted = true
			err = client.Send(ctx, project, chat, prompt, func(event daemon.Event) error {
				if !emit(backendMsg{event: &event}) {
					return ctx.Err()
				}
				return nil
			})
		}
		emit(backendMsg{client: client, project: project, chat: chat, done: true, err: err, attempted: attempted})
	}()
	return m, waitBackend(ch)
}

func (m model) updateBackend(v backendMsg) (tea.Model, tea.Cmd) {
	if v.generation != m.backend.generation || !m.backend.active {
		return m, nil
	}
	if v.stopped {
		m.backend.stopping = false
		var httpErr *daemon.HTTPError
		if errors.As(v.err, &httpErr) && httpErr.StatusCode == 404 && (m.backend.streamEnded || m.backend.stopAccepted) {
			m = m.finishBackend()
			m.status = "Run stopped"
		} else if v.err != nil {
			m.status = "Stop failed or not started: " + v.err.Error()
		} else {
			m.backend.stopAccepted = true
			m.status = "Stop requested; waiting for confirmation"
			return m.pollStop()
		}
		return m, nil
	}
	if v.client != nil {
		m.backend.client, m.backend.project, m.backend.chat = v.client, v.project, v.chat
	}
	if v.event != nil {
		var data struct {
			Type, Channel, Delta, Content, Error, Message, Name, ID string
			Arguments, Result                                       json.RawMessage
		}
		if err := json.Unmarshal(v.event.Data, &data); err != nil {
			m.status = "Invalid daemon event: " + err.Error()
		} else {
			kind := data.Type
			if kind == "" {
				kind = v.event.Type
			}
			switch kind {
			case "assistant_delta":
				if data.Channel == "" || data.Channel == "content" {
					m.streaming = append(m.streaming, []rune(data.Delta)...)
					m.revealed = len(m.streaming)
				}
			case "assistant_end":
				if len(m.streaming) == 0 {
					m.streaming = []rune(data.Content)
				}
				m = m.flushBackend()
			case "tool_start":
				m = m.flushBackend()
				tool := recordedTool{Name: data.Name, Arguments: data.Arguments, Status: "running"}
				if encoded, err := json.Marshal(tool); err == nil {
					m.messages = append(m.messages, message{"TOOLCALL", string(encoded)})
				}
			case "tool_result":
				result := string(data.Result)
				if data.Error != "" {
					result = data.Error + "\n" + result
				}
				m.messages = append(m.messages, message{"RESULT", data.Name + "\n" + result})
			case "error":
				text := data.Error
				if text == "" {
					text = data.Message
				}
				if text == "" {
					text = "Unknown daemon error"
				}
				m.status = "Daemon error: " + text
				m.messages = append(m.messages, message{"NOTICE", m.status})
			case "chat_snapshot":
				var snapshot struct{ Chat struct{ Status string } }
				if json.Unmarshal(v.event.Data, &snapshot) == nil && snapshot.Chat.Status != "running" {
					m.backend.terminal = true
				}
			case "chat_interrupted":
				m.status = "Interrupted"
			}
		}
	}
	if v.done {
		m = m.flushBackend()
		m.backend.streamEnded = true
		var httpErr *daemon.HTTPError
		if !v.attempted || (errors.As(v.err, &httpErr) && httpErr.StatusCode != 409) || (v.err == nil && m.backend.terminal) {
			m = m.finishBackend()
		} else {
			m.status = "Stream lost; stop the run to confirm it has ended"
		}
		if v.err != nil {
			if !m.backend.active {
				m.status = "Daemon error: " + v.err.Error()
			}
			m.messages = append(m.messages, message{"NOTICE", "Daemon error: " + v.err.Error()})
		}
		return m, nil
	}
	return m, waitBackend(m.backend.events)
}

func (m model) flushBackend() model {
	if len(m.streaming) > 0 {
		m.messages = append(m.messages, message{"SOLOMON", string(m.streaming)})
		m.streaming = nil
		m.revealed = 0
	}
	return m
}

func (m model) stopBackend() (tea.Model, tea.Cmd) {
	if m.backend.stopping {
		return m, nil
	}
	if m.backend.client == nil || m.backend.chat == "" {
		m.status = "Cannot stop while connecting"
		return m, nil
	}
	m.backend.stopping = true
	m.status = "Stopping…"
	client, project, chat, generation := m.backend.client, m.backend.project, m.backend.chat, m.backend.generation
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := client.Stop(ctx, project, chat)
		return backendMsg{generation: generation, stopped: true, err: err}
	}
}

func (m model) resetBackend() model {
	if m.backend.cancel != nil {
		m.backend.cancel()
	}
	m.backend.generation++
	m.backend.active, m.backend.stopping = false, false
	m.backend.chat = ""
	m.backend.cancel = nil
	m.backend.events = nil
	return m
}

func (m model) finishBackend() model {
	m.backend.active = false
	m.backend.stopping = false
	if m.backend.cancel != nil {
		m.backend.cancel()
	}
	m.backend.cancel = nil
	if m.status == "Connecting to daemon…" {
		m.status = "Run completed"
	}
	return m
}

func (m model) pollStop() (tea.Model, tea.Cmd) {
	m.backend.stopping = true
	client, project, chat, generation := m.backend.client, m.backend.project, m.backend.chat, m.backend.generation
	return m, func() tea.Msg {
		time.Sleep(300 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return backendMsg{generation: generation, stopped: true, err: client.Stop(ctx, project, chat)}
	}
}

func (m model) requestQuit() (tea.Model, tea.Cmd) {
	if m.backend.active {
		m.status = "Stop the active run before exiting"
		return m, nil
	}
	return m, tea.Quit
}
