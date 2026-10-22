package test

import (
	"bytes"
	"context"
	"encoding/json"
	. "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileChangesChatAccumulationAndCommit(t *testing.T) {
	var output bytes.Buffer
	r := &Runtime{Session: &chatstore.Session{Messages: []chatstore.Message{{Role: "user", CheckpointSeq: 1}}}, Out: &output}
	other := &Runtime{Session: &chatstore.Session{}}
	r.RecordChatFileChangesForTest(2, []filechanges.Change{{Name: "src/a.go", Lines: map[string]int{"old": -1, "first": 1}, BeforeExists: true, AfterExists: true}}, nil)
	r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "user", CheckpointSeq: 4})
	other.RecordChatFileChangesForTest(1, []filechanges.Change{{Name: "other.txt", Lines: map[string]int{"foreign": 1}, AfterExists: true}}, nil)
	r.RecordChatFileChangesForTest(5, []filechanges.Change{{Name: "src/a.go", Lines: map[string]int{"first": -1, "second": 1}, BeforeExists: true, AfterExists: true}, {Name: "b.txt", Lines: map[string]int{"new": 1}, AfterExists: true}}, nil)
	r.PrintChatFileChangesForTest(4)
	if got := output.String(); !strings.Contains(got, "2 changed files") || !strings.Contains(got, "a.go") || !strings.Contains(got, "b.txt") || strings.Contains(got, "other.txt") {
		t.Fatalf("summary: %s", got)
	}
	entry := r.Session.FileChanges["src/a.go"]
	if len(entry.AddedLines) != 1 || len(entry.RemovedLines) != 1 || entry.RemovedLines["old"] != 1 || entry.AddedLines["second"] != 1 {
		t.Fatalf("accumulation: %+v", entry)
	}
	output.Reset()
	r.PrintChatFileChangesForTest(6)
	if output.Len() != 0 {
		t.Fatal("summary leaked into a later unmodified turn")
	}
	encoded, err := json.Marshal(r.Session)
	if err != nil {
		t.Fatal(err)
	}
	var resumed chatstore.Session
	if err := json.Unmarshal(encoded, &resumed); err != nil {
		t.Fatal(err)
	}
	r.Session = &resumed
	r.RecordChatFileChangesForTest(7, nil, []filechanges.Commit{{Name: "src/a.go", Clean: true}})
	if len(r.Session.FileChanges) != 1 {
		t.Fatalf("partial commit cleared unrelated pending edits: %+v", r.Session.FileChanges)
	}
	r.RecordChatFileChangesForTest(8, nil, []filechanges.Commit{{Name: "b.txt", Clean: true}})
	if len(r.Session.FileChanges) != 0 {
		t.Fatal("committed edits remain")
	}
}

func TestFileChangesSameFileAcrossChatsAndPartialCommit(t *testing.T) {
	a := &Runtime{Session: &chatstore.Session{}}
	b := &Runtime{Session: &chatstore.Session{}}
	a.RecordChatFileChangesForTest(1, []filechanges.Change{{Name: "shared.txt", BeforeExists: true, AfterExists: true, Lines: map[string]int{"a-old": -1, "a-new": 1, "pending": 1}}}, nil)
	b.RecordChatFileChangesForTest(1, []filechanges.Change{{Name: "shared.txt", BeforeExists: true, AfterExists: true, Lines: map[string]int{"b-old": -1, "b-new": 1}}}, nil)
	a.RecordChatFileChangesForTest(2, nil, []filechanges.Commit{{Name: "shared.txt", Lines: map[string]int{"a-old": -1, "a-new": 1}}})
	entry := a.Session.FileChanges["shared.txt"]
	if len(entry.AddedLines) != 1 || len(entry.RemovedLines) != 0 || entry.AddedLines["pending"] != 1 {
		t.Fatalf("partial commit lost pending lines: %+v", entry)
	}
	if len(b.Session.FileChanges["shared.txt"].AddedLines) != 1 || len(b.Session.FileChanges["shared.txt"].RemovedLines) != 1 {
		t.Fatal("commit reset another chat")
	}
	a.RecordChatFileChangesForTest(3, []filechanges.Change{{Name: "shared.txt", BeforeExists: true, AfterExists: true, Lines: map[string]int{"pending": -1, "final": 1}}}, nil)
	entry = a.Session.FileChanges["shared.txt"]
	if len(entry.AddedLines) != 1 || len(entry.RemovedLines) != 0 || entry.AddedLines["final"] != 1 {
		t.Fatalf("other chat changes bled into the same path: %+v", entry)
	}
}

func TestFileChangesMovedLinesAndUndo(t *testing.T) {
	r := &Runtime{Session: &chatstore.Session{}}
	moved := filechanges.Change{Name: "move.txt", BeforeExists: true, AfterExists: true, AddedLines: map[string]int{"moved": 1}, RemovedLines: map[string]int{"moved": 1}}
	r.RecordChatFileChangesForTest(1, []filechanges.Change{moved}, nil)
	entry := r.Session.FileChanges["move.txt"]
	if entry.AddedLines["moved"] != 1 || entry.RemovedLines["moved"] != 1 {
		t.Fatalf("move disappeared: %+v", entry)
	}
	r.RecordChatFileChangesForTest(2, []filechanges.Change{moved}, nil)
	if len(r.Session.FileChanges) != 0 {
		t.Fatal("undo left pending changes")
	}
}

func TestFileChangesSavedOnSpecificAssistantTurn(t *testing.T) {
	r := &Runtime{Session: &chatstore.Session{Messages: []chatstore.Message{{Role: "user", CheckpointSeq: 1}, {Role: "assistant", Content: "First response"}}}}
	r.RecordChatFileChangesForTest(2, []filechanges.Change{{Name: "a.txt", AfterExists: true, Lines: map[string]int{"first": 1}}}, nil)
	if err := r.SaveTurnFileChangesForTest(1); err != nil {
		t.Fatal(err)
	}
	first := r.Session.Messages[1].FileChanges
	if len(first) != 1 || first[0].Path != "a.txt" || first[0].Added != 1 {
		t.Fatalf("first turn: %+v", first)
	}
	r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "user", CheckpointSeq: 4}, chatstore.Message{Role: "assistant", Content: "Read only response"})
	if err := r.SaveTurnFileChangesForTest(4); err != nil {
		t.Fatal(err)
	}
	if len(r.Session.Messages[3].FileChanges) != 0 {
		t.Fatal("summary leaked into non-editing turn")
	}
	r.RecordChatFileChangesForTest(5, []filechanges.Change{{Name: "b.txt", AfterExists: true, Lines: map[string]int{"second": 1}}}, nil)
	if err := r.SaveTurnFileChangesForTest(4); err != nil {
		t.Fatal(err)
	}
	if len(r.Session.Messages[3].FileChanges) != 2 || len(r.Session.Messages[1].FileChanges) != 1 {
		t.Fatal("turn snapshots not isolated")
	}
	r.RecordChatFileChangesForTest(6, nil, []filechanges.Commit{{Name: "a.txt", Clean: true}, {Name: "b.txt", Clean: true}})
	if err := r.SaveTurnFileChangesForTest(4); err != nil {
		t.Fatal(err)
	}
	if !r.Session.Messages[3].FileChangesRecorded || len(r.Session.Messages[3].FileChanges) != 0 || len(first) != 1 {
		t.Fatal("commit did not clear current pending summary")
	}
}

func TestFileChangesXDGCommandReachesSavedAssistantMessage(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte("XDG_DESKTOP_DIR=\"$HOME/Desktop\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &Runtime{ProjRoot: home, Cfg: config.EmptyRoot(), Out: &bytes.Buffer{}, Session: &chatstore.Session{Messages: []chatstore.Message{{Role: "user", CheckpointSeq: 0}, {Role: "assistant", CheckpointSeq: 5, Content: "Created"}}}}
	env := r.ToolEnvForInvocationForTest(tooling.Invocation{CheckpointSeq: 4})
	env.AllowDeferredTools = true
	env.ActivateInstructionsFromShellCommand = nil
	command := `desktop="$(xdg-user-dir DESKTOP 2>/dev/null || printf '%s/Desktop' "$HOME")"; mkdir -p "$desktop" || exit 1; target="$desktop/test.txt"; if [ -e "$target" ]; then n=1; while [ -e "$desktop/test-$n.txt" ]; do n=$((n+1)); done; target="$desktop/test-$n.txt"; fi; printf 'File di test.\n' > "$target" && printf 'Creato: %s\n' "$target"`
	raw, _ := json.Marshal(map[string]any{"command": command, "intent": "verify Desktop command reaches assistant metadata"})
	if _, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "shell", Args: raw}); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveTurnFileChangesForTest(0); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(r.Session)
	if err != nil {
		t.Fatal(err)
	}
	var saved chatstore.Session
	if err := json.Unmarshal(encoded, &saved); err != nil {
		t.Fatal(err)
	}
	summary := saved.Messages[1].FileChanges
	if !saved.Messages[1].FileChangesRecorded || len(summary) != 1 || summary[0].Path != "Desktop/test.txt" || summary[0].Added != 1 {
		t.Fatalf("missing assistant summary: %s", encoded)
	}
}

func TestFileChangesResumedOneSidedLedger(t *testing.T) {
	for _, initial := range []string{
		`{"file_changes":{"a.txt":{"added_lines":{"new":1},"after_exists":true}}}`,
		`{"file_changes":{"a.txt":{"removed_lines":{"old":1},"before_exists":true,"after_exists":true}}}`,
	} {
		var session chatstore.Session
		if err := json.Unmarshal([]byte(initial), &session); err != nil {
			t.Fatal(err)
		}
		r := &Runtime{Session: &session}
		// Commits of lines absent from a one-sided ledger must not write to nil maps.
		r.RecordChatFileChangesForTest(1, nil, []filechanges.Commit{{Name: "a.txt", Lines: map[string]int{"absent-add": 1, "absent-remove": -1}}})
		r.RecordChatFileChangesForTest(2, []filechanges.Change{{Name: "a.txt", BeforeExists: true, AfterExists: true, AddedLines: map[string]int{"next": 1}, RemovedLines: map[string]int{"old": 1}}}, nil)
		if got := r.Session.FileChanges["a.txt"].AddedLines["next"]; got != 1 {
			t.Fatalf("new line count = %d", got)
		}
	}
}
