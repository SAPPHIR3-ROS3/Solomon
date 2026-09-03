package config

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

type modelVisibilityUpdate struct {
	provider string
	model    string
	enabled  bool
}

// ModelVisibilityQueue serializes visibility updates without blocking callers on disk I/O.
type ModelVisibilityQueue struct {
	mu      sync.Mutex
	ready   *sync.Cond
	pending []modelVisibilityUpdate
	active  bool
	write   func(string, string, bool) error
}

// NewModelVisibilityQueue starts a queue using the supplied persistence function.
func NewModelVisibilityQueue(write func(string, string, bool) error) *ModelVisibilityQueue {
	q := &ModelVisibilityQueue{write: write}
	q.ready = sync.NewCond(&q.mu)
	go q.run()
	return q
}

// Enqueue validates and schedules one visibility update.
func (q *ModelVisibilityQueue) Enqueue(providerName, modelID string, enabled bool) error {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return fmt.Errorf("provider and model are required")
	}
	if q == nil || q.write == nil {
		return fmt.Errorf("model visibility queue is not configured")
	}
	q.mu.Lock()
	q.pending = append(q.pending, modelVisibilityUpdate{provider: providerName, model: modelID, enabled: enabled})
	q.ready.Signal()
	q.mu.Unlock()
	return nil
}

// Wait blocks until all updates currently in the queue have been persisted.
func (q *ModelVisibilityQueue) Wait() {
	if q == nil {
		return
	}
	q.mu.Lock()
	for q.active || len(q.pending) > 0 {
		q.ready.Wait()
	}
	q.mu.Unlock()
}

func (q *ModelVisibilityQueue) run() {
	for {
		q.mu.Lock()
		for len(q.pending) == 0 {
			q.ready.Wait()
		}
		update := q.pending[0]
		q.pending = q.pending[1:]
		q.active = true
		q.mu.Unlock()

		if err := q.write(update.provider, update.model, update.enabled); err != nil {
			log.Printf("persist model visibility: %v", err)
		}

		q.mu.Lock()
		q.active = false
		if len(q.pending) == 0 {
			q.ready.Broadcast()
		}
		q.mu.Unlock()
	}
}

var defaultModelVisibilityQueue = NewModelVisibilityQueue(UpdateModelVisibility)

// QueueModelVisibility schedules a visibility update on the process-wide writer queue.
func QueueModelVisibility(providerName, modelID string, enabled bool) error {
	return defaultModelVisibilityQueue.Enqueue(providerName, modelID, enabled)
}
