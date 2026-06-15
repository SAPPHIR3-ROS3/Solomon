package agentruntime

import (
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type NestedRunConfig struct {
	SysPromptPath    string
	Task             string
	ResumeID         string
	RunInBackground  bool
	ReasoningEffort  string
	ParentChatID     string
	ParentToolCallID string
	ToolCall         chatstore.ToolCall
	SpawnTime        time.Time
	Origin           string
	ProjectHex       string
	SysPrompt        string
}

type NestedRunResult struct {
	Output    string
	SubchatID string
	Status    string
}

type activeNestedState struct {
	subchatID    string
	origin       string
	parentChatID string
	projectHex   string
}

type subagentRunHandle struct {
	cancel func()
	done   chan struct{}
}

type subagentRegistry struct {
	mu      sync.Mutex
	runs    map[string]*subagentRunHandle
	active  *chatstore.ActiveSubagentsFile
}

var (
	globalSubagentRegistry subagentRegistry
	activeUserSessionsMu   sync.Mutex
	activeUserSessions     int
)

func init() {
	globalSubagentRegistry.runs = make(map[string]*subagentRunHandle)
}

func registerActiveUserSession() {
	activeUserSessionsMu.Lock()
	activeUserSessions++
	activeUserSessionsMu.Unlock()
}

func unregisterActiveUserSession() {
	activeUserSessionsMu.Lock()
	if activeUserSessions > 0 {
		activeUserSessions--
	}
	activeUserSessionsMu.Unlock()
}

func hasActiveUserSession() bool {
	activeUserSessionsMu.Lock()
	n := activeUserSessions
	activeUserSessionsMu.Unlock()
	return n > 0
}

func (reg *subagentRegistry) loadActiveFile() error {
	f, err := chatstore.ReadActiveSubagents()
	if err != nil {
		return err
	}
	reg.active = f
	return nil
}

func (reg *subagentRegistry) persistActive() error {
	if reg.active == nil {
		reg.active = &chatstore.ActiveSubagentsFile{}
	}
	return chatstore.WriteActiveSubagents(reg.active)
}

func (reg *subagentRegistry) isRunning(id string) bool {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	_, ok := reg.runs[id]
	if ok {
		return true
	}
	if reg.active == nil {
		return false
	}
	for _, e := range reg.active.Agents {
		if e.ID == id && chatstore.SubSessionRunning(e.Status) {
			return true
		}
	}
	return false
}

func (reg *subagentRegistry) upsertActiveEntry(e chatstore.ActiveSubagentEntry) error {
	if err := reg.loadActiveFile(); err != nil {
		return err
	}
	found := false
	for i := range reg.active.Agents {
		if reg.active.Agents[i].ID == e.ID {
			reg.active.Agents[i] = e
			found = true
			break
		}
	}
	if !found {
		reg.active.Agents = append(reg.active.Agents, e)
	}
	return reg.persistActive()
}

func (reg *subagentRegistry) removeActiveEntry(id string) error {
	if err := reg.loadActiveFile(); err != nil {
		return err
	}
	out := reg.active.Agents[:0]
	for _, e := range reg.active.Agents {
		if e.ID != id {
			out = append(out, e)
		}
	}
	reg.active.Agents = out
	return reg.persistActive()
}

func (reg *subagentRegistry) registerRun(id string, cancel func()) *subagentRunHandle {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	h := &subagentRunHandle{cancel: cancel, done: make(chan struct{})}
	reg.runs[id] = h
	return h
}

func (reg *subagentRegistry) finishRun(id string) {
	reg.mu.Lock()
	if h, ok := reg.runs[id]; ok {
		close(h.done)
		delete(reg.runs, id)
	}
	reg.mu.Unlock()
	_ = reg.removeActiveEntry(id)
}

func (reg *subagentRegistry) stopRun(id string) {
	reg.mu.Lock()
	h, ok := reg.runs[id]
	reg.mu.Unlock()
	if ok && h.cancel != nil {
		h.cancel()
	}
}

func (reg *subagentRegistry) reconcileOnStartup() {
	f, err := chatstore.ReadActiveSubagents()
	if err != nil || f == nil {
		return
	}
	changed := false
	for i, e := range f.Agents {
		if e.Status == chatstore.SubStatusRunning {
			f.Agents[i].Status = chatstore.SubStatusPaused
			changed = true
		}
	}
	if changed {
		_ = chatstore.WriteActiveSubagents(f)
	}
}
