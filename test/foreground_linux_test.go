package test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/gorilla/websocket"
)

func TestTerminalForegroundActivityAndExecutable(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/__solomon/terminal?path="+url.QueryEscape(t.TempDir()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	wait := func(running bool, title string) {
		t.Helper()
		connection.SetReadDeadline(time.Now().Add(8 * time.Second))
		for {
			var message struct {
				Type    string `json:"type"`
				Running bool   `json:"running"`
				Title   string `json:"title"`
			}
			if err := connection.ReadJSON(&message); err != nil {
				t.Fatal(err)
			}
			if message.Type == "solomon-status" && message.Running == running && strings.Contains(message.Title, title) {
				return
			}
		}
	}
	wait(false, "sh")
	if err := connection.WriteMessage(websocket.BinaryMessage, []byte("sleep 2\r")); err != nil {
		t.Fatal(err)
	}
	wait(true, "sleep")
	wait(false, "sh")
	if err := connection.WriteMessage(websocket.BinaryMessage, []byte("sleep 2 &\r")); err != nil {
		t.Fatal(err)
	}
	wait(false, "sh")
	if err := connection.WriteMessage(websocket.BinaryMessage, []byte("exec sleep 2\r")); err != nil {
		t.Fatal(err)
	}
	wait(true, "sleep")
}
