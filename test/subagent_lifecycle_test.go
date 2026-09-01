package test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type lifecycleBackend struct {
	started chan struct{}
	mu      sync.Mutex
	calls   int
	efforts []string
}

func (b *lifecycleBackend) Protocol() llm.Protocol { return llm.ProtocolOpenAI }

func (b *lifecycleBackend) StreamTurn(ctx context.Context, req llm.TurnRequest, _ io.Writer, _ llm.StreamOpts) (llm.AssistantTurnResult, error) {
	b.mu.Lock()
	b.calls++
	b.efforts = append(b.efforts, req.ReasoningEffort)
	call := b.calls
	b.mu.Unlock()
	if call == 1 {
		close(b.started)
		<-ctx.Done()
		return llm.AssistantTurnResult{}, ctx.Err()
	}
	return llm.AssistantTurnResult{Content: "resumed"}, nil
}

func (b *lifecycleBackend) StreamText(context.Context, llm.SimpleCompletionRequest, io.Writer, llm.StreamOpts) (string, llm.UsageStats, error) {
	return "", llm.UsageStats{}, errors.New("not used")
}
func (b *lifecycleBackend) CompleteText(context.Context, llm.SimpleCompletionRequest) (string, error) {
	return "", errors.New("not used")
}
func (b *lifecycleBackend) ListModels(context.Context) ([]string, error) { return nil, nil }

func TestSubagentBackgroundStopAndResumeLifecycle(t *testing.T) {
	logging.LogInit(logging.ERROR_LOG_LEVEL)
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projHex := "lifecycle-project"
	prov := &config.Provider{Name: "test", BaseURL: "http://127.0.0.1:9", APIKey: "key", AuthKind: config.AuthKindAPIKey}
	cfg := &config.Root{
		Current:                 config.Current{Provider: "test", Model: "test-model"},
		Providers:               map[string]*config.Provider{"test": prov},
		SubagentReasoningEffort: "high",
	}
	r := agentruntime.NewTestRuntime(cfg, prov, projHex, t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
	stopCursorSidecar(t)
	b := &lifecycleBackend{started: make(chan struct{})}
	r.Backend = b

	res, err := r.RunSubagentToolForTest(context.Background(), agentruntime.NestedRunConfig{
		Task:            "background work",
		RunInBackground: true,
		Origin:          chatstore.SubOriginParent,
		ProjectHex:      projHex,
		ToolCall:        chatstore.ToolCall{ID: "call-1", Name: "subagent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.ControlSubagentForTest(res.SubchatID, "stop"); err != nil {
			t.Logf("cleanup subagent: %v", err)
		}
	})
	select {
	case <-b.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background subagent did not start")
	}

	if err := r.ControlSubagentForTest(res.SubchatID, "stop"); err != nil {
		t.Fatal(err)
	}
	sess, err := chatstore.FindSubSessionByID(projHex, res.SubchatID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != chatstore.SubStatusPaused {
		t.Fatalf("after stop status=%q", sess.Status)
	}

	if err := r.ControlSubagentForTest(res.SubchatID, "resume"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		sess, err = chatstore.FindSubSessionByID(projHex, res.SubchatID)
		if err != nil {
			t.Fatal(err)
		}
		if sess.Status == chatstore.SubStatusDone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after resume status=%q", sess.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.mu.Lock()
	efforts := append([]string(nil), b.efforts...)
	b.mu.Unlock()
	if len(efforts) != 2 || efforts[0] != "high" || efforts[1] != "high" {
		t.Fatalf("reasoning efforts=%v", efforts)
	}
}

func TestSubagentSynchronousCancellationCleansLifecycle(t *testing.T) {
	logging.LogInit(logging.ERROR_LOG_LEVEL)
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projHex := "sync-lifecycle-project"
	prov := &config.Provider{Name: "test", BaseURL: "http://127.0.0.1:9", APIKey: "key", AuthKind: config.AuthKindAPIKey}
	cfg := &config.Root{
		Current:   config.Current{Provider: "test", Model: "test-model"},
		Providers: map[string]*config.Provider{"test": prov},
	}
	r := agentruntime.NewTestRuntime(cfg, prov, projHex, t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
	b := &lifecycleBackend{started: make(chan struct{})}
	r.Backend = b

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := r.RunSubagentToolForTest(ctx, agentruntime.NestedRunConfig{
			Task:       "foreground work",
			Origin:     chatstore.SubOriginParent,
			ProjectHex: projHex,
			ToolCall:   chatstore.ToolCall{ID: "sync-call-1", Name: "subagent"},
		})
		resultCh <- err
	}()
	select {
	case <-b.started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("synchronous subagent did not start")
	}
	sessions, err := chatstore.ListSubSessions(projHex)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		cancel()
		t.Fatalf("subagent sessions before cancellation=%+v", sessions)
	}
	subchatID := sessions[0].ID
	activeBeforeStop, err := chatstore.ReadActiveSubagents()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if len(activeBeforeStop.Agents) != 0 {
		cancel()
		t.Fatalf("synchronous subagent was registered as background work: %+v", activeBeforeStop.Agents)
	}
	cancel()
	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("synchronous cancellation returned nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("synchronous subagent did not stop")
	}

	sess, err := chatstore.FindSubSessionByID(projHex, subchatID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != chatstore.SubStatusPaused {
		t.Fatalf("after cancellation status=%q", sess.Status)
	}
	active, err := chatstore.ReadActiveSubagents()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range active.Agents {
		if entry.ProjectHex == projHex {
			t.Fatalf("cancelled synchronous subagent remains active: %+v", entry)
		}
	}
}
