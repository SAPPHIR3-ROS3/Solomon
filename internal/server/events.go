package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"net/http"
	"os"
	"sync"
	"time"
)

type chatRun struct {
	ctx            context.Context
	startedAt      time.Time
	cancel         context.CancelFunc
	done           chan struct{}
	finishOnce     sync.Once
	stateMu        sync.Mutex
	finished       bool
	events         []cievents.Event
	subscribers    map[uint64]chatRunSubscriber
	nextSubscriber uint64
}

type chatRunSubscriber struct {
	ctx    context.Context
	events chan cievents.Event
}

func newChatRun(parent context.Context) *chatRun {
	ctx, cancel := context.WithCancel(parent)
	return &chatRun{
		ctx:         ctx,
		startedAt:   time.Now().UTC(),
		cancel:      cancel,
		done:        make(chan struct{}),
		events:      make([]cievents.Event, 0, 64),
		subscribers: make(map[uint64]chatRunSubscriber),
	}
}

func (r *chatRun) Emit(ev cievents.Event) {
	if ev["type"] == "error" {
		if message, ok := ev["error"].(string); ok {
			logging.RecordNotification("error", message)
		}
	}
	r.stateMu.Lock()
	if r.finished {
		r.stateMu.Unlock()
		return
	}
	value := make(cievents.Event, len(ev)+1)
	for key, item := range ev {
		value[key] = item
	}
	value["seq"] = int64(len(r.events) + 1)
	r.events = append(r.events, value)
	type subscriberEntry struct {
		id  uint64
		sub chatRunSubscriber
	}
	subscribers := make([]subscriberEntry, 0, len(r.subscribers))
	for id, subscriber := range r.subscribers {
		subscribers = append(subscribers, subscriberEntry{id: id, sub: subscriber})
	}
	r.stateMu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case <-r.ctx.Done():
			return
		case subscriber.sub.events <- value:
		case <-subscriber.sub.ctx.Done():
			r.unsubscribe(subscriber.id)
		}
	}
}

func (r *chatRun) StreamMode() bool { return true }

func (r *chatRun) Events() []cievents.Event { return nil }

func (r *chatRun) FlushReport(cievents.ReportMeta, int, string, string, any) error { return nil }

func (r *chatRun) subscribe(ctx context.Context, startingAfter int64) ([]cievents.Event, <-chan cievents.Event, <-chan struct{}, func()) {
	subscriber := chatRunSubscriber{ctx: ctx, events: make(chan cievents.Event, 256)}
	r.stateMu.Lock()
	replay := make([]cievents.Event, 0, len(r.events))
	for _, ev := range r.events {
		seq, _ := ev["seq"].(int64)
		if seq > startingAfter {
			replay = append(replay, ev)
		}
	}
	id := r.nextSubscriber
	r.nextSubscriber++
	if !r.finished {
		r.subscribers[id] = subscriber
	}
	done := r.done
	r.stateMu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() { r.unsubscribe(id) })
	}
	return replay, subscriber.events, done, unsubscribe
}

func (r *chatRun) unsubscribe(id uint64) {
	r.stateMu.Lock()
	delete(r.subscribers, id)
	r.stateMu.Unlock()
}

func (r *chatRun) finish() {
	r.finishOnce.Do(func() {
		r.stateMu.Lock()
		r.finished = true
		close(r.done)
		r.stateMu.Unlock()
	})
}

func (a *chatAPI) handleChatEvents(w http.ResponseWriter, r *http.Request, projectID, chatID string) {
	startingAfter, err := parseStartingAfter(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	key := projectID + "\x00" + chatID
	if run := a.getRun(key); run != nil {
		a.streamChatRun(w, r, run, startingAfter)
		return
	}

	sess, readErr := chatstore.ReadSession(projectID, chatID)
	if readErr != nil {
		status := http.StatusInternalServerError
		if errors.Is(readErr, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, readErr)
		return
	}
	writeSSEHeaders(w)
	_ = writeSSE(w, apiEvent("chat_snapshot", map[string]any{
		"chat": apiChatFromSession(projectID, sess, nil),
	}))
}

func (a *chatAPI) streamChatRun(w http.ResponseWriter, r *http.Request, run *chatRun, startingAfter int64) {
	replay, events, done, unsubscribe := run.subscribe(r.Context(), startingAfter)
	defer unsubscribe()
	writeSSEHeaders(w)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for _, event := range replay {
		if err := writeSSE(w, event); err != nil {
			return
		}
	}
	for {
		select {
		case event := <-events:
			if err := writeSSE(w, event); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeSSEComment(w, "keep-alive"); err != nil {
				return
			}
		case <-done:
			for {
				select {
				case event := <-events:
					if err := writeSSE(w, event); err != nil {
						return
					}
				default:
					return
				}
			}
		case <-r.Context().Done():
			return
		}
	}
}

type httpEventSink struct {
	ctx     context.Context
	events  chan cievents.Event
	once    sync.Once
	stateMu sync.RWMutex
	closed  bool
}

func newHTTPEventSink(ctx context.Context) *httpEventSink {
	return &httpEventSink{ctx: ctx, events: make(chan cievents.Event, 256)}
}

func (s *httpEventSink) Emit(ev cievents.Event) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.closed {
		return
	}
	select {
	case s.events <- ev:
	case <-s.ctx.Done():
	}
}

func (s *httpEventSink) StreamMode() bool { return true }

func (s *httpEventSink) Events() []cievents.Event { return nil }

func (s *httpEventSink) FlushReport(cievents.ReportMeta, int, string, string, any) error { return nil }

func (s *httpEventSink) Close() {
	s.once.Do(func() {
		s.stateMu.Lock()
		s.closed = true
		close(s.events)
		s.stateMu.Unlock()
	})
}

func (s *httpEventSink) EventsChannel() <-chan cievents.Event { return s.events }

func apiEvent(typ string, fields map[string]any) cievents.Event {
	event := cievents.Event{"v": cievents.SchemaVersion, "type": typ, "ts": time.Now().UTC().Format(time.RFC3339Nano)}
	for key, value := range fields {
		event[key] = value
	}
	return event
}

func writeSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

func writeSSE(w http.ResponseWriter, event cievents.Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeSSEComment(w http.ResponseWriter, comment string) error {
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
