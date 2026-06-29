package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestExport_requiresTarget(t *testing.T) {
	d := testDeps(&chatstore.Session{Messages: []chatstore.Message{{Role: "user", Content: "hi"}}})
	err := commands.Export(d, []string{"/export"})
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestExportChatBasename(t *testing.T) {
	if got := commands.ExportChatBasenameForTest(&chatstore.Session{Title: "Refactor Auth!!!"}); got != "refactor-auth" {
		t.Fatalf("slug: %q", got)
	}
	id := "abc123def456"
	if got := commands.ExportChatBasenameForTest(&chatstore.Session{ID: id}); got != id {
		t.Fatalf("id basename: %q", got)
	}
}

func TestPlanExportPath_counterStartsAtOne(t *testing.T) {
	root := t.TempDir()
	day := time.Date(2025, 6, 29, 12, 0, 0, 0, time.UTC)
	dateDir := filepath.Join(root, "2025-06-29")
	if err := os.MkdirAll(dateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "refactor-auth.md"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := commands.PlanExportPathForTest(root, day, "refactor-auth", false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(plan.AbsolutePath) != "refactor-auth-1.md" {
		t.Fatalf("expected refactor-auth-1.md, got %s", plan.AbsolutePath)
	}
}

func TestPlanExportPath_lastRejectsExisting(t *testing.T) {
	root := t.TempDir()
	day := time.Date(2025, 6, 29, 12, 0, 0, 0, time.UTC)
	dateDir := filepath.Join(root, "2025-06-29")
	if err := os.MkdirAll(dateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "my-chat.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := commands.PlanExportPathForTest(root, day, "my-chat", true)
	if err == nil || !strings.Contains(err.Error(), "already exported") {
		t.Fatalf("expected already exported error, got %v", err)
	}
}

func TestExport_currentWritesMarkdown(t *testing.T) {
	root := t.TempDir()
	sess := &chatstore.Session{
		ID:            "chat-id-hex",
		Title:         "My Chat",
		CreatedAt:     time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC),
		LastMessageAt: time.Date(2025, 6, 1, 11, 0, 0, 0, time.UTC),
		Messages: []chatstore.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	var out bytes.Buffer
	d := testDeps(sess)
	d.Out = &out
	d.Cfg = &config.Root{
		Current:   config.Current{Provider: "p", Model: "m"},
		Providers: map[string]*config.Provider{"p": {Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}},
		Export:    config.Export{Path: root},
	}
	if err := commands.Export(d, []string{"/export", "current"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "exported chat to") {
		t.Fatalf("missing confirmation: %q", out.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, time.Now().UTC().Format("2006-01-02")))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one export file, got %d", len(entries))
	}
	body, err := os.ReadFile(filepath.Join(root, time.Now().UTC().Format("2006-01-02"), entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"# My Chat", "## Metadata", "## Transcript", "**You:**", "**m (none):**", "hello", "world"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestWriteMarkdownExport_imagesAppendix(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "one.png")
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	if err := os.WriteFile(imgPath, png, 0o600); err != nil {
		t.Fatal(err)
	}
	sess := &chatstore.Session{
		Title: "img chat",
		Messages: []chatstore.Message{
			{Role: "user", Content: "see [img-1]"},
		},
		ImageFiles: map[int]string{1: imgPath},
	}
	var buf bytes.Buffer
	meta := commands.MarkdownExportMetaForTest("img chat", "/tmp/proj", "model")
	if err := commands.WriteMarkdownExportForTest(&buf, meta, sess, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "see [img-1]") {
		t.Fatalf("placeholder missing: %s", out)
	}
	if !strings.Contains(out, "## Images") || !strings.Contains(out, "[img-1] = data:image/png;base64,") {
		t.Fatalf("images appendix missing: %s", out)
	}
}
