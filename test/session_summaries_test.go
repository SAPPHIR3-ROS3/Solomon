package test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestSessionSummariesSkipPayloadAndRefreshCache(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	project := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path, err := chatstore.SessionPath(project, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	directory, _ := chatstore.ChatsDir(project)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	session := chatstore.Session{ID: "test", Title: "Before", CreatedAt: now.Add(-time.Hour), Messages: []chatstore.Message{{Role: "user", CreatedAt: now, Content: "private prompt"}, {Role: "assistant", CreatedAt: now.Add(time.Minute), Content: "private answer"}}, FileChanges: map[string]chatstore.FileChangeLedger{"a.go": {AddedLines: map[string]int{"private code": 1}}}}
	write := func() {
		data, _ := json.Marshal(session)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	for i := 0; i < 2; i++ {
		summaries, err := chatstore.ListSummaries(project, 10)
		if err != nil || len(summaries) != 1 {
			t.Fatalf("summaries: %v %v", summaries, err)
		}
		summary := summaries[0]
		if summary.Title != session.Title || !summary.LastUserMessageAt.Equal(now) || len(summary.Messages) != 0 || len(summary.FileChanges) != 0 {
			t.Fatalf("summary: %+v", summary)
		}
		session.Title = "After cache invalidation"
		write()
	}
}

func TestSessionSummaryCacheDetectsAtomicReplacement(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	project := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path, err := chatstore.SessionPath(project, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	directory, _ := chatstore.ChatsDir(project)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(-time.Hour)
	write := func(title string) {
		data, _ := json.Marshal(chatstore.Session{ID: "test", Title: title})
		if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path+".tmp", stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			t.Fatal(err)
		}
	}
	write("First")
	if _, err := chatstore.ListSummaries(project, 10); err != nil {
		t.Fatal(err)
	}
	write("Other")
	summaries, err := chatstore.ListSummaries(project, 10)
	if err != nil || len(summaries) != 1 || summaries[0].Title != "Other" {
		t.Fatalf("stale summary: %+v %v", summaries, err)
	}
}
