package test

import (
	"slices"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/desktopgit"
)

func TestParseDesktopGitHistory(t *testing.T) {
	output := strings.Join([]string{
		"0123456789012345678901234567890123456789\x000123456\x00Ada Lovelace\x002026-08-10T12:30:00+02:00\x00Merge feature\x00HEAD -> main, tag: v1.0\x00aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x00aaaaaaa\x00Grace Hopper\x002026-08-09T09:15:00+02:00\x00Ship it\x00\x00cccccccccccccccccccccccccccccccccccccccc",
	}, "\n")

	commits := desktopgit.ParseHistory([]byte(output))
	if len(commits) != 2 {
		t.Fatalf("parseDesktopGitHistory() returned %d commits, want 2", len(commits))
	}
	if got, want := commits[0].Subject, "Merge feature"; got != want {
		t.Fatalf("first subject = %q, want %q", got, want)
	}
	if got, want := len(commits[0].Parents), 2; got != want {
		t.Fatalf("first parent count = %d, want %d", got, want)
	}
	if got, want := commits[0].Refs, []string{"HEAD -> main", "tag: v1.0"}; !slices.Equal(got, want) {
		t.Fatalf("first refs = %#v, want %#v", got, want)
	}
	if got, want := commits[1].Author, "Grace Hopper"; got != want {
		t.Fatalf("second author = %q, want %q", got, want)
	}
}

func TestParseDesktopGitStatus(t *testing.T) {
	output := []byte("M\x00gui/src/App.tsx\x00R100\x00old-name.tsx\x00new-name.tsx\x00")
	status := desktopgit.ParseStatus(output)
	if got, want := status["gui/src/App.tsx"], "M"; got != want {
		t.Fatalf("modified status = %q, want %q", got, want)
	}
	if got, want := status["new-name.tsx"], "R"; got != want {
		t.Fatalf("renamed status = %q, want %q", got, want)
	}
	if _, ok := status["old-name.tsx"]; ok {
		t.Fatal("old rename path should not be listed")
	}
}
