package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solomon-tui-prototypes/daemon"
)

func TestDaemonWorkflow(t *testing.T) {
	home := t.TempDir()
	started := time.Now().UTC().Truncate(time.Second)
	var address string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "server": daemon.State{PID: 123, URL: address, Home: home, StartedAt: started}})
		case "/__solomon/projects":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || body["path"] != home {
				t.Errorf("project request: %s %#v", r.Method, body)
			}
			fmt.Fprintf(w, `{"project":{"id":"project","path":%q}}`, home)
		case "/__solomon/projects/project/chats":
			fmt.Fprint(w, `{"id":"chat","projectID":"project"}`)
		case "/__solomon/projects/project/chats/chat/messages":
			if r.Header.Get("Accept") != "text/event-stream" {
				t.Error("missing SSE accept")
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			fmt.Fprint(w, ": ping\r\nevent: update\r\nid: 4\r\ndata: {\"type\":\r\ndata: \"delta\"}\r\n\r\n")
		case "/__solomon/projects/project/chats/chat/stop":
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	address = server.URL
	dir := filepath.Join(home, "run", "server")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(daemon.State{PID: 123, URL: address, Home: home, StartedAt: started})
	if err := os.WriteFile(filepath.Join(dir, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := daemon.Discover(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.CurrentProject(ctx, home)
	if err != nil || p.ID != "project" {
		t.Fatalf("project: %+v %v", p, err)
	}
	chat, err := c.CreateChat(ctx, p.ID, "")
	if err != nil || chat.ID != "chat" {
		t.Fatalf("chat: %+v %v", chat, err)
	}
	count := 0
	err = c.Send(ctx, p.ID, chat.ID, "hello", func(event daemon.Event) error {
		count++
		if event.Type != "update" || event.ID != "4" || !strings.Contains(string(event.Data), "delta") {
			t.Errorf("event: %+v", event)
		}
		return nil
	})
	if err != nil || count != 1 {
		t.Fatalf("stream: %d %v", count, err)
	}
	if err = c.Stop(ctx, p.ID, chat.ID); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(daemon.State{PID: 124, URL: address, Home: home, StartedAt: started})
	os.WriteFile(filepath.Join(dir, "state.json"), data, 0600)
	if _, err = daemon.Discover(ctx, home); err == nil {
		t.Fatal("accepted mismatched identity")
	}
}

func TestDaemonRejectsRemoteState(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "run", "server")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(daemon.State{PID: 123, URL: "http://example.com:64000", StartedAt: time.Now()})
	if err := os.WriteFile(filepath.Join(dir, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := daemon.Discover(context.Background(), home); err == nil {
		t.Fatal("accepted remote URL")
	}
}
