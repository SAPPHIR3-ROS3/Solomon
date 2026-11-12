package test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

func TestRunAgentTurns_compactsBeforeNextUserRequest(t *testing.T) {
	for _, ephemeral := range []bool{false, true} {
		t.Run(map[bool]string{false: "persisted", true: "ephemeral"}[ephemeral], func(t *testing.T) {
			backend := &turnScriptBackend{
				protocol: llm.ProtocolOpenAI,
				turns: []llm.AssistantTurnResult{
					{Content: "done", FinishReason: llm.FinishReasonStop, Usage: llm.UsageStats{PromptTokens: 5000}},
					{Content: "next answer", FinishReason: llm.FinishReasonStop},
				},
				texts: []string{"summary of previous work"},
			}
			rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
				r.EphemeralSession = ephemeral
				r.CompactionThresholdTokens = 5000
			})
			if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
				t.Fatal(err)
			}
			if backend.textN != 0 || backend.turnN != 1 {
				t.Fatalf("calls after stop: summary=%d turn=%d", backend.textN, backend.turnN)
			}
			pending := chatstore.Message{Role: "user", Content: "visible request", APIContent: "exact expanded request"}
			rt.Session.Messages = append(rt.Session.Messages, pending)
			if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
				t.Fatal(err)
			}
			if backend.textN != 1 || backend.turnN != 2 {
				t.Fatalf("calls: summary=%d turn=%d", backend.textN, backend.turnN)
			}
			msgs := backend.requests[1].Messages
			if len(msgs) != 2 || !chatstore.IsCompactionSummary(msgs[0]) || msgs[1].Content != pending.Content || msgs[1].APIContent != pending.APIContent {
				t.Fatalf("next request lost summary or exact user message: %+v", msgs)
			}
			if len(rt.Session.UncompactedRaw) != 1 {
				t.Fatal("original history was not archived")
			}
		})
	}
}

func TestApplyCompactionWithPendingMessagesPreservesImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.png")
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, 0600); err != nil {
		t.Fatal(err)
	}
	pending := chatstore.Message{Role: "user", Content: images.Placeholder(1) + " inspect this", APIContent: "expanded request"}
	sess := &chatstore.Session{
		Messages:   []chatstore.Message{{Role: "assistant", Content: "old answer"}, pending},
		ImageFiles: map[int]string{1: path},
	}
	chatstore.RepairSessionMalformedImages(sess)
	pending = sess.Messages[1]
	chatstore.ApplyCompactionWithPendingMessages(sess, "summary", time.Now(), []chatstore.Message{pending})
	if sess.ImageFiles[1] != path || sess.Messages[1].Content != pending.Content || sess.Messages[1].APIContent != pending.APIContent {
		t.Fatalf("pending image or message changed: %+v", sess)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRunAgentTurns_compactsAfterToolResults(t *testing.T) {
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{ToolCalls: []llm.AssistantToolCall{{ID: "tc1", Name: "searchTools", Arguments: searchToolsArgs()}}, Usage: llm.UsageStats{PromptTokens: 5000}},
			{Content: "done", FinishReason: llm.FinishReasonStop},
		},
		texts: []string{"tool search completed"},
	}
	rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
		r.EphemeralSession = true
		r.CompactionThresholdTokens = 100
	})
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if backend.textN != 1 || backend.turnN != 2 || !chatstore.IsCompactionSummary(backend.requests[1].Messages[0]) {
		t.Fatalf("calls: summary=%d turn=%d", backend.textN, backend.turnN)
	}
	archived := rt.Session.UncompactedRaw[0].Messages
	if archived[len(archived)-1].Role != "tool" || archived[len(archived)-1].ToolCallID != "tc1" {
		t.Fatal("compaction ran before tool completion")
	}
}

func TestRunAgentTurns_autoCompactionFailureStopsRequest(t *testing.T) {
	for _, summary := range []string{"", "   "} {
		backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, texts: []string{summary}}
		sess := &chatstore.Session{ID: "oversized", Messages: []chatstore.Message{
			{Role: "user", Content: "old question"},
			{Role: "assistant", Content: "old answer", PromptTokens: 5000},
			{Role: "user", Content: "new question"},
		}}
		rt := newTurnLoopRuntime(t, backend, sess, func(r *agentruntime.Runtime) {
			r.CompactionThresholdTokens = 100
		})
		err := rt.RunAgentTurnsForTest(context.Background())
		if err == nil || !strings.Contains(err.Error(), "auto-compact") || backend.turnN != 0 {
			t.Fatalf("error=%v turn calls=%d", err, backend.turnN)
		}
		if len(rt.Session.Messages) != 3 || len(rt.Session.UncompactedRaw) != 0 {
			t.Fatal("failed compaction changed history")
		}
	}
}

type compactInterruptBackend struct {
	*turnScriptBackend
	textStarted chan struct{}
}

func (b *compactInterruptBackend) StreamText(ctx context.Context, req llm.SimpleCompletionRequest, contentOut io.Writer, opts llm.StreamOpts) (string, llm.UsageStats, error) {
	close(b.textStarted)
	<-ctx.Done()
	return "", llm.UsageStats{}, ctx.Err()
}

func TestRunAgentTurns_interruptDuringAutoCompaction(t *testing.T) {
	backend := &compactInterruptBackend{
		turnScriptBackend: &turnScriptBackend{protocol: llm.ProtocolOpenAI},
		textStarted:       make(chan struct{}),
	}
	sess := &chatstore.Session{ID: "compact-interrupt", Messages: []chatstore.Message{
		{Role: "user", Content: "old question"},
		{Role: "assistant", Content: "old answer", PromptTokens: 5000},
		{Role: "user", Content: "new question"},
	}}
	rt := newTurnLoopRuntime(t, backend, sess, func(r *agentruntime.Runtime) {
		r.CompactionThresholdTokens = 100
	})
	errCh := make(chan error, 1)
	go func() { errCh <- rt.RunAgentTurnsForTest(context.Background()) }()
	select {
	case <-backend.textStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("auto-compaction did not start")
	}
	agentruntime.StopAgentGenerationForTest()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("interrupt during auto-compaction returned error, want generation stopped outcome: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("turn loop did not stop after interrupt")
	}
	if backend.turnN != 0 {
		t.Fatalf("StreamTurn calls=%d, want 0 (request must not start after interrupted compaction)", backend.turnN)
	}
}
