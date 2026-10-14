//go:build !windows

package test

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/gorilla/websocket"
)

func TestTerminalExplicitCloseTerminatesProcess(t *testing.T) {
	for _, transport := range []string{"websocket", "http"} {
		t.Run(transport, func(t *testing.T) {
			t.Setenv("SHELL", "/bin/sh")
			server, stop := startServerForTest(t, serverruntime.Options{})
			defer stop()
			directory := t.TempDir()
			connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/__solomon/terminal?path="+url.QueryEscape(directory), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var session struct {
				ID string `json:"id"`
			}
			if err := connection.ReadJSON(&session); err != nil || session.ID == "" {
				t.Fatalf("terminal handshake: id=%q err=%v", session.ID, err)
			}
			// Record the real shell PID to verify it exits, not just its socket.
			if err := connection.WriteMessage(websocket.BinaryMessage, []byte("printf '%s' $$ > shell.pid\r")); err != nil {
				t.Fatal(err)
			}
			var pid int
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(filepath.Join(directory, "shell.pid"))
				pid, _ = strconv.Atoi(string(data))
				if pid > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("terminal shell did not record its PID")
			}
			if transport == "websocket" {
				if err := connection.WriteJSON(map[string]string{"type": "close"}); err != nil {
					t.Fatal(err)
				}
			} else {
				// Explicit closure must also work after the browser disconnects.
				connection.Close()
				request, err := http.NewRequest(http.MethodDelete, server.URL+"/__solomon/terminal?session_id="+url.QueryEscape(session.ID), nil)
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
			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("terminal process is still running after explicit close")
		})
	}
}
