package test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type turnScriptBackend struct {
	protocol llm.Protocol
	turns    []llm.AssistantTurnResult
	turnErr  []error
	texts    []string
	mu       sync.Mutex
	turnN    int
	textN    int
}

func (b *turnScriptBackend) Protocol() llm.Protocol { return b.protocol }

func (b *turnScriptBackend) StreamTurn(ctx context.Context, req llm.TurnRequest, contentOut io.Writer, opts llm.StreamOpts) (llm.AssistantTurnResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.turnN
	b.turnN++
	if i < len(b.turnErr) && b.turnErr[i] != nil {
		return llm.AssistantTurnResult{}, b.turnErr[i]
	}
	if i >= len(b.turns) {
		return llm.AssistantTurnResult{}, errors.New("unexpected StreamTurn")
	}
	turn := b.turns[i]
	if contentOut != nil && turn.Content != "" {
		_, _ = io.WriteString(contentOut, turn.Content)
	}
	return turn, nil
}

func (b *turnScriptBackend) StreamText(ctx context.Context, req llm.SimpleCompletionRequest, contentOut io.Writer, opts llm.StreamOpts) (string, llm.UsageStats, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.textN
	b.textN++
	if i >= len(b.texts) {
		return "", llm.UsageStats{}, errors.New("unexpected StreamText")
	}
	return b.texts[i], llm.UsageStats{}, nil
}

func (b *turnScriptBackend) CompleteText(ctx context.Context, req llm.SimpleCompletionRequest) (string, error) {
	return "", errors.New("not implemented")
}

func (b *turnScriptBackend) ListModels(ctx context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func newTurnLoopRuntime(t *testing.T, backend llm.CompletionBackend, sess *chatstore.Session, tune func(*agentruntime.Runtime)) *agentruntime.Runtime {
	t.Helper()
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projRoot := t.TempDir()
	prov := &config.Provider{Name: "test", BaseURL: "http://127.0.0.1:9", APIKey: "k", AuthKind: config.AuthKindAPIKey}
	cfg := &config.Root{
		Current:   config.Current{Provider: "test", Model: "test-model"},
		Providers: map[string]*config.Provider{"test": prov},
	}
	if sess == nil {
		sess = &chatstore.Session{
			ID:       "turn-loop-test",
			Messages: []chatstore.Message{{Role: "user", Content: "hello"}},
		}
	}
	rt := agentruntime.NewTestRuntime(cfg, prov, testProjectHex, projRoot, sess, io.Discard)
	rt.Backend = backend
	if tune != nil {
		tune(rt)
	}
	return rt
}

func searchToolsArgs() string {
	return `{"query":"shell"}`
}

func shellEchoArgs() string {
	return `{"command":"echo tool-ok","intent":"turn loop test"}`
}

func TestRunAgentTurns_simpleAssistantReply(t *testing.T) {
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns:    []llm.AssistantTurnResult{{Content: "hello back"}},
	}
	rt := newTurnLoopRuntime(t, backend, nil, nil)
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rt.Session.Messages) != 2 {
		t.Fatalf("messages=%d", len(rt.Session.Messages))
	}
	if rt.Session.Messages[1].Role != "assistant" || rt.Session.Messages[1].Content != "hello back" {
		t.Fatalf("assistant=%+v", rt.Session.Messages[1])
	}
}

func TestRunAgentTurns_toolCallRoundTrip(t *testing.T) {
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{ToolCalls: []llm.AssistantToolCall{{ID: "tc1", Name: "searchTools", Arguments: searchToolsArgs()}}},
			{Content: "done"},
		},
	}
	rt := newTurnLoopRuntime(t, backend, nil, nil)
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rt.Session.Messages) != 4 {
		t.Fatalf("messages=%d", len(rt.Session.Messages))
	}
	if rt.Session.Messages[1].Role != "assistant" || len(rt.Session.Messages[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool turn=%+v", rt.Session.Messages[1])
	}
	if rt.Session.Messages[2].Role != "tool" || rt.Session.Messages[2].ToolCallID != "tc1" {
		t.Fatalf("tool result=%+v", rt.Session.Messages[2])
	}
	if rt.Session.Messages[3].Content != "done" {
		t.Fatalf("final assistant=%+v", rt.Session.Messages[3])
	}
	if backend.turnN != 2 {
		t.Fatalf("StreamTurn calls=%d", backend.turnN)
	}
}

func TestRunAgentTurns_interruptDuringTool(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	restore := agentruntime.SetExecToolHookForTest(func(ctx context.Context, inv tooling.Invocation) (any, error) {
		close(started)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
			return map[string]any{"output": "late"}, nil
		}
	})
	defer restore()
	defer close(release)

	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{ToolCalls: []llm.AssistantToolCall{{ID: "tc1", Name: "shell", Arguments: shellEchoArgs()}}},
		},
	}
	rt := newTurnLoopRuntime(t, backend, nil, nil)
	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.RunAgentTurnsForTest(context.Background())
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("tool did not start")
	}
	agentruntime.StopAgentGenerationForTest()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunAgentTurns: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("turn loop did not stop")
	}
	foundTool := false
	for _, m := range rt.Session.Messages {
		if m.Role == "tool" && m.ToolCallID == "tc1" && strings.Contains(m.Content, "Generation stopped") {
			foundTool = true
			break
		}
	}
	if !foundTool {
		t.Fatalf("synthetic tool result missing: %+v", rt.Session.Messages)
	}
}

func TestRunAgentTurns_streamError(t *testing.T) {
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turnErr:  []error{errors.New("stream failed")},
	}
	rt := newTurnLoopRuntime(t, backend, nil, nil)
	err := rt.RunAgentTurnsForTest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stream failed") {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestRunAgentTurns_ephemeralAutoCompaction(t *testing.T) {
	summary := "compact summary for turn loop test"
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{Content: "before compact", Usage: llm.UsageStats{PromptTokens: 5000}},
			{Content: "after compact"},
		},
		texts: []string{summary},
	}
	rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
		r.EphemeralSession = true
		r.CompactionThresholdTokens = 100
	})
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if backend.textN != 1 {
		t.Fatalf("StreamText calls=%d", backend.textN)
	}
	if backend.turnN != 2 {
		t.Fatalf("StreamTurn calls=%d", backend.turnN)
	}
	if len(rt.Session.Messages) != 2 {
		t.Fatalf("messages=%d", len(rt.Session.Messages))
	}
	if !strings.Contains(rt.Session.Messages[0].Content, summary) {
		t.Fatalf("compact body=%q", rt.Session.Messages[0].Content)
	}
	if rt.Session.Messages[1].Content != "after compact" {
		t.Fatalf("final assistant=%+v", rt.Session.Messages[1])
	}
}
