//go:build !windows

package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTerminalExplicitCloseTerminatesProcess(t *testing.T) {
	for _, transport := range []string{"websocket", "http"} {
		t.Run(transport, func(t *testing.T) {
			ctx := make(chan struct{})
			manager := newTerminalManager(ctx)
			defer close(ctx)
			defer manager.CloseAll()
			session, err := manager.create(t.TempDir(), 80, 24, "/bin/sh", []string{"-c", "printf READY; exec sleep 60"})
			if err != nil {
				t.Fatal(err)
			}
			api := newTerminalAPI(manager)
			server := httptest.NewServer(http.HandlerFunc(api.handleWebSocket))
			defer server.Close()
			if transport == "websocket" {
				connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?session_id="+session.id, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer connection.Close()
				if err := connection.WriteJSON(map[string]string{"type": "close"}); err != nil {
					t.Fatal(err)
				}
			} else {
				request, err := http.NewRequest(http.MethodDelete, server.URL+"?session_id="+session.id, nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusNoContent {
					t.Fatalf("close returned %d", response.StatusCode)
				}
			}
			select {
			case <-session.done:
			case <-time.After(5 * time.Second):
				t.Fatal("terminal process is still running after explicit close")
			}
			if session.Info().Running {
				t.Fatal("closed terminal still reports running")
			}
			if _, err := manager.Get(session.id); !errors.Is(err, errTerminalSessionNotFound) {
				t.Fatalf("closed session is still registered: %v", err)
			}
		})
	}
}
