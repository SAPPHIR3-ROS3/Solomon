package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

type depthBackend struct {
	lifecycleBackend
	calls     int
	rejection string
}

func (b *depthBackend) StreamTurn(_ context.Context, req llm.TurnRequest, _ io.Writer, _ llm.StreamOpts) (llm.AssistantTurnResult, error) {
	for _, message := range req.Messages {
		if message.Role == "tool" {
			if strings.Contains(message.Content, "recursion limit reached") {
				b.rejection = message.Content
			}
			return llm.AssistantTurnResult{Content: "finished"}, nil
		}
	}
	b.calls++
	return llm.AssistantTurnResult{ToolCalls: []llm.AssistantToolCall{{ID: fmt.Sprintf("depth-%d", b.calls), Name: "subagent", Arguments: `{"task":"delegate again","intent":"inspect nested task"}`}}}, nil
}

func TestSubagentRecursionDepthLimit(t *testing.T) {
	for _, limit := range []int{0, 2} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			t.Setenv("SOLOMON_HOME", t.TempDir())
			provider := &config.Provider{Name: "test", AuthKind: config.AuthKindAPIKey, APIKey: "key"}
			cfg := &config.Root{Current: config.Current{Provider: "test", Model: "test-model"}, Providers: map[string]*config.Provider{"test": provider}, SubagentMaxDepth: limit}
			r := agentruntime.NewTestRuntime(cfg, provider, "depth-project", t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
			backend := &depthBackend{}
			r.Backend = backend
			_, err := r.RunSubagentToolForTest(context.Background(), agentruntime.NestedRunConfig{Task: "start", ProjectHex: "depth-project", Origin: chatstore.SubOriginParent})
			if err != nil {
				t.Fatal(err)
			}
			expected := config.SubagentDepthLimit(cfg)
			if backend.calls != expected || !strings.Contains(backend.rejection, fmt.Sprintf("maximum %d", expected)) {
				t.Fatalf("calls=%d rejection=%s", backend.calls, backend.rejection)
			}
			sessions, err := chatstore.ListSubSessions("depth-project")
			if err != nil {
				t.Fatal(err)
			}
			if len(sessions) != expected {
				t.Fatalf("created %d subagents, want %d", len(sessions), expected)
			}
			for _, session := range sessions {
				if session.Depth > 1 {
					parent, err := chatstore.FindSubSessionByID("depth-project", session.ParentSubchatID)
					if err != nil || parent.Depth != session.Depth-1 {
						t.Fatalf("invalid caller at depth %d: parent=%+v err=%v", session.Depth, parent, err)
					}
				}
				if session.Depth < 1 || session.Depth > expected || session.Status != chatstore.SubStatusDone {
					t.Fatalf("invalid session: %+v", session)
				}
			}
		})
	}
}

func TestSubagentResumeKeepsPersistedDepth(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	cfg := &config.Root{SubagentMaxDepth: 2}
	r := agentruntime.NewTestRuntime(cfg, nil, "depth-project", t.TempDir(), &chatstore.Session{ID: "parent"}, io.Discard)
	session := &chatstore.SubSession{ID: "deep", Depth: 3, Status: chatstore.SubStatusDone, Origin: chatstore.SubOriginParent, ProjectHex: "depth-project"}
	if err := chatstore.WriteSubSession("depth-project", session); err != nil {
		t.Fatal(err)
	}
	_, err := r.RunSubagentToolForTest(context.Background(), agentruntime.NestedRunConfig{ResumeID: "deep", Task: "resume"})
	if err == nil || !strings.Contains(err.Error(), "recursion limit reached") {
		t.Fatalf("resume error=%v", err)
	}
	saved, err := chatstore.FindSubSessionByID("depth-project", "deep")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Depth != 3 || saved.Status != chatstore.SubStatusDone || len(saved.Messages) != 0 {
		t.Fatalf("rejected resume mutated session: %+v", saved)
	}
	for _, background := range []bool{false, true} {
		_, err := r.RunSubagentToolForTest(context.Background(), agentruntime.NestedRunConfig{
			Task: "too deep", Depth: 3, RunInBackground: background,
		})
		if err == nil || !strings.Contains(err.Error(), "recursion limit reached") {
			t.Fatalf("background=%v bypassed limit: %v", background, err)
		}
	}
}

func TestSubagentDepthAdvancedSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("user_name='Test'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Advanced["subagent_max_depth"] != 4 {
		t.Fatalf("default=%v", settings.Advanced)
	}
	_, err = config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"subagent_max_depth": json.RawMessage("6")}})
	if err != nil {
		t.Fatal(err)
	}
	settings, err = config.ReadGUISettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Advanced["subagent_max_depth"] != 6 {
		t.Fatalf("saved=%v", settings.Advanced)
	}
	for _, value := range []string{"0", "-1", "33", "1.5", "null", "true"} {
		if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: map[string]json.RawMessage{"subagent_max_depth": json.RawMessage(value)}}); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
