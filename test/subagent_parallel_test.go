package test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type parallelSubagentBackend struct {
	lifecycleBackend
	started    chan string
	release    map[string]chan struct{}
	resumed    chan []chatstore.Message
	background bool
}

func (b *parallelSubagentBackend) StreamTurn(ctx context.Context, req llm.TurnRequest, _ io.Writer, _ llm.StreamOpts) (llm.AssistantTurnResult, error) {
	task := req.Messages[0].Content
	if task == "parent" {
		if len(req.Messages) > 1 {
			b.resumed <- req.Messages
			return llm.AssistantTurnResult{Content: "parent complete"}, nil
		}
		return llm.AssistantTurnResult{ToolCalls: []llm.AssistantToolCall{
			{ID: "first", Name: "subagent", Arguments: `{"task":"first","intent":"delegate first task"}`},
			{ID: "second", Name: "subagent", Arguments: fmt.Sprintf(`{"task":"second","intent":"delegate second task","run_in_background":%t}`, b.background)},
			{ID: "follow-up", Name: "searchTools", Arguments: `{"query":"files","intent":"find file tools"}`},
		}}, nil
	}
	b.started <- task
	select {
	case <-b.release[task]:
		if task == "second" {
			return llm.AssistantTurnResult{}, fmt.Errorf("second task failed")
		}
		return llm.AssistantTurnResult{Content: "first complete"}, nil
	case <-ctx.Done():
		return llm.AssistantTurnResult{}, ctx.Err()
	}
}

func TestNestedSubagentBatchWaitsForEverySynchronousCall(t *testing.T) {
	t.Run("synchronous siblings", func(t *testing.T) { testNestedSubagentBatch(t, false) })
	t.Run("mixed synchronous and background siblings", func(t *testing.T) { testNestedSubagentBatch(t, true) })
}

func testNestedSubagentBatch(t *testing.T, background bool) {
	t.Helper()
	setSolomonHome(t, t.TempDir())
	const project = "parallel-subagents"
	provider := &config.Provider{Name: "test", AuthKind: config.AuthKindAPIKey, APIKey: "key"}
	cfg := &config.Root{Current: config.Current{Provider: "test", Model: "test-model"}, Providers: map[string]*config.Provider{"test": provider}}
	r := agentruntime.NewTestRuntime(cfg, provider, project, t.TempDir(), &chatstore.Session{ID: "chat"}, io.Discard)
	backend := &parallelSubagentBackend{
		started:    make(chan string, 2),
		release:    map[string]chan struct{}{"first": make(chan struct{}), "second": make(chan struct{})},
		resumed:    make(chan []chatstore.Message, 1),
		background: background,
	}
	r.Backend = backend
	followUp := make(chan struct{}, 1)
	defer agentruntime.SetExecToolHookForTest(func(_ context.Context, inv tooling.Invocation) (any, error) {
		if inv.ToolCallID != "follow-up" {
			return nil, fmt.Errorf("unexpected tool call %q", inv.ToolCallID)
		}
		followUp <- struct{}{}
		return map[string]bool{"ok": true}, nil
	})()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan struct{})
	var parent agentruntime.NestedRunResult
	var parentErr error
	go func() {
		defer close(done)
		parent, parentErr = r.RunSubagentToolForTest(ctx, agentruntime.NestedRunConfig{Task: "parent"})
	}()
	defer func() {
		cancel()
		<-done
		if background {
			stopCtx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			if err := agentruntime.StopChatSubagents(stopCtx, project, "chat"); err != nil {
				t.Error(err)
			}
		}
	}()

	started := make(map[string]bool)
	for range 2 {
		select {
		case task := <-backend.started:
			started[task] = true
		case <-ctx.Done():
			t.Fatalf("children did not start concurrently: %v", started)
		}
	}
	if !started["first"] || !started["second"] {
		t.Fatalf("unexpected children: %v", started)
	}

	secondResult := "second task failed"
	if background {
		secondResult = `"status":"running"`
	} else {
		close(backend.release["second"])
	}
	select {
	case <-backend.resumed:
		t.Fatal("parent resumed while the first child was still running")
	case <-followUp:
		t.Fatal("next tool ran while the first child was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(backend.release["first"])

	select {
	case msgs := <-backend.resumed:
		if len(msgs) != 5 || msgs[2].ToolCallID != "first" || !strings.Contains(msgs[2].Content, "first complete") ||
			msgs[3].ToolCallID != "second" || !strings.Contains(msgs[3].Content, secondResult) || msgs[4].ToolCallID != "follow-up" {
			t.Fatalf("unexpected parent messages: %+v", msgs)
		}
	case <-ctx.Done():
		t.Fatal("parent did not resume after both children finished")
	}

	select {
	case <-done:
		if parentErr != nil {
			t.Fatal(parentErr)
		}
	case <-ctx.Done():
		t.Fatal("parent did not finish")
	}
	sessions, err := chatstore.ListSubSessions(project)
	if err != nil || len(sessions) != 3 {
		t.Fatalf("expected parent and two children: count=%d, err=%v", len(sessions), err)
	}
	for _, session := range sessions {
		if session.ID != parent.SubchatID && (session.ParentSubchatID != parent.SubchatID || session.Depth != 2) {
			t.Fatalf("invalid child hierarchy: %+v", session)
		}
	}
}
