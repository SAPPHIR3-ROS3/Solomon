package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type completionQueue struct {
	mu      sync.Mutex
	active  int
	pending []string
	changed chan struct{}
}

func (q *completionQueue) begin() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active++
	if q.changed == nil {
		q.changed = make(chan struct{})
	}
}

func (q *completionQueue) finish(message string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active--
	q.pending = append(q.pending, message)
	if q.changed != nil {
		close(q.changed)
	}
	q.changed = make(chan struct{})
}

func (q *completionQueue) take(ctx context.Context, wait bool) ([]string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		q.mu.Lock()
		if len(q.pending) > 0 || !wait || q.active == 0 {
			messages := q.pending
			q.pending = nil
			q.mu.Unlock()
			return messages, nil
		}
		changed := q.changed
		q.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (r *Runtime) finishSubagent(cfg NestedRunConfig, result NestedRunResult, err error) {
	task := strings.TrimSpace(cfg.Task)
	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "failed"
	}
	message := fmt.Sprintf("[subagent-completed] Subagent task %q finished with status %s. Subchat: %s.", task, status, result.SubchatID)
	if err != nil {
		message += "\nError: " + err.Error()
	}
	if output := strings.TrimSpace(result.Output); output != "" {
		text := []rune(output)
		if len(text) > 16000 {
			text = text[len(text)-16000:]
		}
		message += "\nResult:\n" + string(text)
	}
	r.subagentCompletions.finish(message)
}

func (r *Runtime) DeliverSubagentCompletions(ctx context.Context, wait bool) (bool, error) {
	messages, err := r.subagentCompletions.take(ctx, wait)
	if err != nil || len(messages) == 0 {
		return false, err
	}
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		for _, content := range messages {
			s.Messages = append(s.Messages, chatstore.Message{Role: "user", Content: content, GUIHidden: true, CreatedAt: time.Now()})
		}
	})
	return true, r.persistSession()
}
