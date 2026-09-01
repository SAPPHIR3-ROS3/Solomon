package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	servercli "github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/gorilla/websocket"
	terminal "golang.org/x/term"
)

var errDaemonTerminalExited = errors.New("daemon terminal exited")

type daemonTerminalMessage struct {
	Data    string `json:"data"`
	ID      string `json:"id"`
	Running bool   `json:"running"`
	Seq     uint64 `json:"seq"`
	Type    string `json:"type"`
}

// daemonTUIRequested keeps the explicit tui/attach aliases while making the
// normal interactive CLI a daemon client. Headless commands are dispatched
// before this check and remain local/machine-oriented.
func daemonTUIRequested(args []string) bool {
	if len(args) == 1 {
		return true
	}
	if args[1] == "tui" || args[1] == "attach" {
		return true
	}
	if len(args) != 2 {
		return false
	}
	// Keep the existing machine and maintenance commands on their dedicated
	// paths. A single directory argument is the long-standing shorthand for
	// starting Solomon in that workspace, so it must use the same daemon-backed
	// TUI as the no-argument form.
	switch args[1] {
	case "add", "remove", "exec", "temp", "server", "version", "upgrade", "init", "sandbox-worker", "templates":
		return false
	default:
		return true
	}
}

func daemonTUIWorkingDirectory(args []string) (string, error) {
	start := 1
	if len(args) > 1 && (args[1] == "tui" || args[1] == "attach") {
		start = 2
	}
	if len(args) > start+1 {
		return "", errors.New("usage: solomon [tui|attach] [directory]")
	}
	value := ""
	if len(args) == start+1 {
		value = expandPathArg(args[start])
	}
	if value == "" {
		return os.Getwd()
	}
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("TUI directory is not a directory: %s", absolute)
	}
	return absolute, nil
}

func runDaemonTUI(args []string) error {
	workingDirectory, err := daemonTUIWorkingDirectory(args)
	if err != nil {
		return err
	}
	if !terminal.IsTerminal(int(os.Stdin.Fd())) || !terminal.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("interactive TUI requires a terminal")
	}
	baseURL, err := ensureDaemonForTUI()
	if err != nil {
		return err
	}
	state, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("enter raw terminal mode: %w", err)
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), state) }()

	input := make(chan []byte, 64)
	go readTUIInput(input)
	var sessionID string
	var outputSeq uint64
	for {
		connection, err := dialDaemonTerminal(baseURL, workingDirectory, sessionID, outputSeq)
		if err != nil {
			if waitErr := waitForDaemonReconnect(750 * time.Millisecond); waitErr != nil {
				return err
			}
			continue
		}
		sendTerminalResize(connection)
		readDone := make(chan error, 1)
		go func() {
			readDone <- readDaemonTerminal(connection, os.Stdout, &sessionID, &outputSeq)
		}()

		reconnect := false
		resizeTicker := time.NewTicker(500 * time.Millisecond)
		for !reconnect {
			select {
			case data, ok := <-input:
				if !ok {
					resizeTicker.Stop()
					_ = connection.Close()
					return nil
				}
				if err := connection.WriteMessage(websocket.BinaryMessage, data); err != nil {
					reconnect = true
				}
			case err := <-readDone:
				resizeTicker.Stop()
				_ = connection.Close()
				if errors.Is(err, errDaemonTerminalExited) {
					return nil
				}
				reconnect = true
			case <-resizeTicker.C:
				sendTerminalResize(connection)
			}
		}
		resizeTicker.Stop()
		if waitErr := waitForDaemonReconnect(250 * time.Millisecond); waitErr != nil {
			return waitErr
		}
	}
}

func ensureDaemonForTUI() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SOLOMON_SERVER_URL")); configured != "" {
		return strings.TrimRight(configured, "/"), nil
	}
	state, err := serverruntime.LoadState()
	if err != nil {
		servercli.Run([]string{"start"})
		state, err = serverruntime.LoadState()
	}
	if err != nil || strings.TrimSpace(state.URL) == "" {
		return "", errors.New("Solomon daemon is unavailable; run solomon server start")
	}
	return strings.TrimRight(state.URL, "/"), nil
}

func dialDaemonTerminal(baseURL, workingDirectory, sessionID string, after uint64) (*websocket.Conn, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported Solomon server URL scheme %q", parsed.Scheme)
	}
	parsed.Path = "/__solomon/terminal"
	query := parsed.Query()
	query.Set("mode", "tui")
	query.Set("path", workingDirectory)
	if sessionID != "" {
		query.Set("session_id", sessionID)
	}
	if after > 0 {
		query.Set("after", fmt.Sprint(after))
	}
	parsed.RawQuery = query.Encode()
	connection, _, err := (&websocket.Dialer{HandshakeTimeout: 10 * time.Second}).Dial(parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func readDaemonTerminal(connection *websocket.Conn, output io.Writer, sessionID *string, outputSeq *uint64) error {
	for {
		messageType, data, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		if messageType == websocket.BinaryMessage {
			_, _ = output.Write(data)
			continue
		}
		var message daemonTerminalMessage
		if err := json.Unmarshal(data, &message); err != nil {
			_, _ = output.Write(data)
			continue
		}
		switch message.Type {
		case "solomon-terminal":
			if message.ID != "" {
				*sessionID = message.ID
			}
		case "solomon-output":
			if message.Seq <= *outputSeq || message.Data == "" {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(message.Data)
			if err != nil {
				return err
			}
			*outputSeq = message.Seq
			_, _ = output.Write(decoded)
		case "solomon-exit":
			return errDaemonTerminalExited
		}
	}
}

func readTUIInput(input chan<- []byte) {
	buffer := make([]byte, 32<<10)
	for {
		count, err := os.Stdin.Read(buffer)
		if count > 0 {
			input <- append([]byte(nil), buffer[:count]...)
		}
		if err != nil {
			close(input)
			return
		}
	}
}

func sendTerminalResize(connection *websocket.Conn) {
	cols, rows, err := terminal.GetSize(int(os.Stdin.Fd()))
	if err != nil || cols <= 0 || rows <= 0 {
		return
	}
	_ = connection.WriteJSON(map[string]any{"cols": cols, "rows": rows, "type": "resize"})
}

func waitForDaemonReconnect(delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
	return nil
}
