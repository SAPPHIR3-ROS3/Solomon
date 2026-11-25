package agentruntime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type NestedRunConfig struct {
	savedTarget      bool
	ParentSubchatID  string
	Depth            int
	SysPromptPath    string
	Task             string
	ResumeID         string
	Interrupt        bool
	RunInBackground  bool
	ReasoningEffort  string
	ParentChatID     string
	ParentToolCallID string
	ToolCall         chatstore.ToolCall
	SpawnTime        time.Time
	Origin           string
	ProjectHex       string
	SysPrompt        string
	RoleProvider     string
	RoleModel        string
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
	projectID, chatID string
	cancel            func()
	done              chan struct{}
}

type subagentRegistry struct {
	mu     sync.Mutex
	runs   map[string]*subagentRunHandle
	active *chatstore.ActiveSubagentsFile
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
	// Serialize the read-modify-write so concurrent lifecycle transitions do not
	// race over activeSubagents.json or its temporary file.
	reg.mu.Lock()
	defer reg.mu.Unlock()

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
	reg.mu.Lock()
	defer reg.mu.Unlock()

	return reg.removeActiveEntryLocked(id)
}

func (reg *subagentRegistry) removeActiveEntryLocked(id string) error {
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

func (reg *subagentRegistry) registerRun(ctx context.Context, sess *chatstore.SubSession, cancel func()) (*subagentRunHandle, error) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, running := reg.runs[sess.ID]; running {
		return nil, fmt.Errorf("subchat %s is running; use /subagent stop first", sess.ID)
	}
	h := &subagentRunHandle{projectID: sess.ProjectHex, chatID: sess.ParentChatID, cancel: cancel, done: make(chan struct{})}
	reg.runs[sess.ID] = h
	return h, nil
}

func StopChatSubagents(ctx context.Context, projectID, chatID string) error {
	reg := &globalSubagentRegistry
	reg.mu.Lock()
	var runs []*subagentRunHandle
	for _, h := range reg.runs {
		if h.projectID == projectID && h.chatID == chatID {
			h.cancel()
			runs = append(runs, h)
		}
	}
	reg.mu.Unlock()
	for _, h := range runs {
		select {
		case <-h.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (reg *subagentRegistry) finishRun(id string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	_ = reg.removeActiveEntryLocked(id)
	if h, ok := reg.runs[id]; ok {
		close(h.done)
		delete(reg.runs, id)
	}
}

func (reg *subagentRegistry) cancelAndWait(id string, timeout time.Duration) error {
	reg.mu.Lock()
	h, ok := reg.runs[id]
	reg.mu.Unlock()
	if !ok {
		return nil
	}
	if h.cancel != nil {
		h.cancel()
	}
	if timeout <= 0 {
		<-h.done
		return nil
	}
	select {
	case <-h.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("subagent %s did not stop within %s", id, timeout)
	}
}

func (reg *subagentRegistry) reconcileOnStartup() {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	f, err := chatstore.ReadActiveSubagents()
	if err != nil || f == nil {
		return
	}
	changed := false
	for i, e := range f.Agents {
		if e.Status != chatstore.SubStatusRunning {
			continue
		}
		if _, live := reg.runs[e.ID]; live {
			continue
		}
		f.Agents[i].Status = chatstore.SubStatusPaused
		if sess, sessErr := chatstore.FindSubSessionByID(e.ProjectHex, e.ID); sessErr == nil && chatstore.SubSessionRunning(sess.Status) {
			sess.Status = chatstore.SubStatusPaused
			_ = chatstore.WriteSubSession(e.ProjectHex, sess)
		}
		changed = true
	}
	if changed {
		_ = chatstore.WriteActiveSubagents(f)
	}
}
