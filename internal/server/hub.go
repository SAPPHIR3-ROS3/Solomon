package server

import (
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
)

const defaultEventBuffer = 512

type streamEvent struct {
	ID   int
	Type string
	Data map[string]any
}

type eventBuffer struct {
	mu     sync.RWMutex
	events []streamEvent
	nextID int
}

func newEventBuffer() *eventBuffer {
	return &eventBuffer{events: make([]streamEvent, 0, defaultEventBuffer)}
}

func (b *eventBuffer) append(typ string, data map[string]any) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	ev := streamEvent{ID: b.nextID, Type: typ, Data: data}
	b.events = append(b.events, ev)
	return ev.ID
}

func (b *eventBuffer) since(lastID int) []streamEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]streamEvent, 0)
	for _, ev := range b.events {
		if ev.ID > lastID {
			out = append(out, ev)
		}
	}
	return out
}

func (b *eventBuffer) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = b.events[:0]
	b.nextID = 0
}

type activeTurn struct {
	responseID   string
	conversation string
	cancel       func()
	done         chan struct{}
	buffer       *eventBuffer
	started      time.Time
}

type Hub struct {
	mu            sync.RWMutex
	active        *activeTurn
	liveConv      string
	subscribers   map[chan streamEvent]struct{}
	subscribersMu sync.Mutex
	store         *responseStore
}

func NewHub(store *responseStore) *Hub {
	return &Hub{subscribers: map[chan streamEvent]struct{}{}, store: store}
}

func (h *Hub) SetLiveConversation(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.liveConv = id
}

func (h *Hub) LiveConversation() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.liveConv
}

func (h *Hub) TurnActive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.active != nil
}

func (h *Hub) ActiveResponseID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.active == nil {
		return ""
	}
	return h.active.responseID
}

func (h *Hub) BeginTurn(responseID, conversation string, cancel func()) *eventBuffer {
	h.mu.Lock()
	defer h.mu.Unlock()
	buf := newEventBuffer()
	h.active = &activeTurn{
		responseID:   responseID,
		conversation: conversation,
		cancel:       cancel,
		done:         make(chan struct{}),
		buffer:       buf,
		started:      time.Now().UTC(),
	}
	h.liveConv = conversation
	return buf
}

func (h *Hub) EndTurn() {
	h.mu.Lock()
	t := h.active
	h.active = nil
	h.mu.Unlock()
	if t != nil {
		close(t.done)
	}
}

func (h *Hub) FinishTurn(status, outputText string) {
	h.mu.Lock()
	t := h.active
	h.active = nil
	h.mu.Unlock()
	if t == nil {
		return
	}
	events := t.buffer.snapshot()
	rec := persistedResponse{
		ID:           t.responseID,
		Object:       "response",
		Conversation: t.conversation,
		Status:       status,
		OutputText:   outputText,
		Events:       events,
		CompletedAt:  time.Now().UTC(),
	}
	if h.store != nil {
		_ = h.store.Put(rec)
	}
	close(t.done)
}

func (h *Hub) GetStored(id string) (*persistedResponse, bool) {
	if h.ActiveResponseID() == id {
		return nil, false
	}
	if h.store == nil {
		return nil, false
	}
	return h.store.Get(id)
}

func (b *eventBuffer) snapshot() []streamEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]streamEvent, len(b.events))
	copy(out, b.events)
	return out
}

func (h *Hub) CancelActive() bool {
	h.mu.RLock()
	t := h.active
	h.mu.RUnlock()
	if t == nil || t.cancel == nil {
		return false
	}
	t.cancel()
	return true
}

func (h *Hub) ActiveBuffer() *eventBuffer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.active == nil {
		return nil
	}
	return h.active.buffer
}

func (h *Hub) WaitTurnDone(responseID string) <-chan struct{} {
	h.mu.RLock()
	t := h.active
	h.mu.RUnlock()
	if t == nil || t.responseID != responseID {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return t.done
}

func (h *Hub) Publish(ev streamEvent) {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *Hub) Subscribe() (chan streamEvent, func()) {
	ch := make(chan streamEvent, 64)
	h.subscribersMu.Lock()
	h.subscribers[ch] = struct{}{}
	h.subscribersMu.Unlock()
	stop := func() {
		h.subscribersMu.Lock()
		delete(h.subscribers, ch)
		h.subscribersMu.Unlock()
		close(ch)
	}
	return ch, stop
}

func cieventToResponseEvents(ev cievents.Event) []streamEvent {
	typ, _ := ev["type"].(string)
	switch typ {
	case cievents.TypeAssistantDelta:
		delta, _ := ev["delta"].(string)
		ch, _ := ev["channel"].(string)
		if ch == cievents.ChannelReasoning {
			return []streamEvent{{Type: "response.reasoning.delta", Data: map[string]any{"delta": delta}}}
		}
		return []streamEvent{{Type: "response.output_text.delta", Data: map[string]any{"delta": delta}}}
	case cievents.TypeToolStart:
		return []streamEvent{{Type: "response.function_call_arguments.delta", Data: map[string]any{
			"name":      ev["name"],
			"arguments": ev["arguments"],
		}}}
	case cievents.TypeToolResult:
		return []streamEvent{{Type: "response.function_call_output", Data: map[string]any{
			"name":   ev["name"],
			"result": ev["result"],
		}}}
	case cievents.TypeError:
		return []streamEvent{{Type: "error", Data: map[string]any{"message": ev["message"]}}}
	default:
		return nil
	}
}
