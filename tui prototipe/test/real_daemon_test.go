package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"solomon-tui-prototypes/daemon"
)

func TestDaemonRealStream(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_REAL_DAEMON") != "1" {
		t.Skip("set SOLOMON_TEST_REAL_DAEMON=1 to use the configured daemon and model")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := daemon.Discover(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	project, err := client.CurrentProject(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := client.CreateChat(ctx, project.ID, "TUI integration verification")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("verification chat: %s", chat.ID)
	gotAnswer := false
	gotFailure := ""
	err = client.Send(ctx, project.ID, chat.ID, "Reply only with the word OK. Do not use tools.", func(event daemon.Event) error {
		var value struct{ Type, Channel, Delta, Content, Message, Error string }
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		if value.Type == "assistant_delta" && value.Channel == "content" && value.Delta != "" {
			gotAnswer = true
		}
		if value.Type == "assistant_end" && value.Content != "" {
			gotAnswer = true
		}
		if value.Type == "error" {
			gotFailure = value.Message + value.Error
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotFailure != "" {
		t.Fatalf("daemon reported error: %s", gotFailure)
	}
	if !gotAnswer {
		t.Fatal("stream ended without assistant content")
	}
}
