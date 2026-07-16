package agentruntime

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

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
	r := NewTestRuntime(cfg, prov, projHex, t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
	b := &lifecycleBackend{started: make(chan struct{})}
	r.Backend = b

	res, err := r.runSubagentTool(context.Background(), NestedRunConfig{
		Task:            "background work",
		RunInBackground: true,
		Origin:          chatstore.SubOriginParent,
		ProjectHex:      projHex,
		ToolCall:        chatstore.ToolCall{ID: "call-1", Name: "subagent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background subagent did not start")
	}

	if err := r.controlSubagent(res.SubchatID, "stop"); err != nil {
		t.Fatal(err)
	}
	sess, err := chatstore.FindSubSessionByID(projHex, res.SubchatID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != chatstore.SubStatusPaused {
		t.Fatalf("after stop status=%q", sess.Status)
	}

	if err := r.controlSubagent(res.SubchatID, "resume"); err != nil {
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
