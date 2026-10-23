package test

import (
	"errors"
	"os"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestTerminalChatsRemainSeparateAndRejectWrongProject(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	const project = "project"
	for _, id := range []string{"first", "second"} {
		if err := chatstore.WriteSession(project, &chatstore.Session{ID: id, Title: id}); err != nil {
			t.Fatal(err)
		}
		if err := chatstore.RememberTerminalChat("terminal-"+id, project, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"first", "second"} {
		session, err := chatstore.ReadTerminalChat("terminal-"+id, project)
		if err != nil || session.ID != id {
			t.Fatalf("terminal %s restored %+v: %v", id, session, err)
		}
	}
	if _, err := chatstore.ReadTerminalChat("terminal-first", "another-project"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cross-project restore: %v", err)
	}
	if err := chatstore.RememberTerminalChat("terminal-first", project, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := chatstore.ReadTerminalChat("terminal-first", project); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty terminal restored old chat: %v", err)
	}
	if err := chatstore.RememberTerminalChat("../../escape", project, "first"); err == nil {
		t.Fatal("accepted invalid terminal ID")
	}
	if _, err := chatstore.ReadTerminalChat("../../escape", project); err == nil {
		t.Fatal("read invalid terminal ID")
	}
}
