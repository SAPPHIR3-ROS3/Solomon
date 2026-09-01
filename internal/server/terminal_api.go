package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type terminalAPI struct {
	manager *terminalManager
}

type terminalControlMessage struct {
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Type string `json:"type"`
}

func newTerminalAPI(manager *terminalManager) *terminalAPI {
	return &terminalAPI{manager: manager}
}

func (a *terminalAPI) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	if !terminalOriginAllowed(r) {
		writeAPIError(w, http.StatusForbidden, errors.New("terminal origin is not allowed"))
		return
	}
	query := r.URL.Query()
	after, err := parseUintQuery(query.Get("after"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid terminal output cursor"))
		return
	}
	cols, err := parseIntQuery(query.Get("cols"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid terminal columns"))
		return
	}
	rows, err := parseIntQuery(query.Get("rows"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid terminal rows"))
		return
	}

	sessionID := strings.TrimSpace(query.Get("session_id"))
	mode := strings.TrimSpace(query.Get("mode"))
	var session *terminalSession
	if sessionID == "" {
		if mode == "tui" {
			session, err = a.manager.CreateTUI(query.Get("path"), cols, rows)
		} else {
			session, err = a.manager.Create(query.Get("path"), cols, rows)
		}
	} else {
		session, err = a.manager.Get(sessionID)
		// A daemon restart invalidates the client's persisted session id. Start a
		// fresh daemon-owned PTY in that case so the client can recover without
		// getting stuck retrying a permanent 404.
		if errors.Is(err, errTerminalSessionNotFound) {
			if mode == "tui" {
				session, err = a.manager.CreateTUI(query.Get("path"), cols, rows)
			} else {
				session, err = a.manager.Create(query.Get("path"), cols, rows)
			}
			after = 0
		}
		if err == nil && session.Info().Running && cols > 0 && rows > 0 {
			err = session.Resize(cols, rows)
		}
	}
	if err != nil {
		status := http.StatusInternalServerError
		writeAPIError(w, status, err)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:     terminalOriginAllowed,
		ReadBufferSize:  32 << 10,
		WriteBufferSize: 32 << 10,
	}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(1 << 20)

	replay, events, done, unsubscribe := session.Subscribe(r.Context(), after)
	defer unsubscribe()
	if err := writeTerminalJSON(connection, map[string]any{
		"cwd":  session.Info().Cwd,
		"seq":  session.Info().Seq,
		"id":   session.Info().ID,
		"type": "solomon-terminal",
	}); err != nil {
		return
	}
	if err := writeTerminalJSON(connection, map[string]any{
		"running": session.Info().Running,
		"type":    "solomon-status",
	}); err != nil {
		return
	}
	for _, output := range replay {
		if err := writeTerminalOutput(connection, output); err != nil {
			return
		}
	}

	inputErrors := make(chan error, 1)
	go func() {
		for {
			messageType, data, readErr := connection.ReadMessage()
			if readErr != nil {
				inputErrors <- readErr
				return
			}
			if messageType == websocket.BinaryMessage {
				if writeErr := session.Write(data); writeErr != nil {
					inputErrors <- writeErr
					return
				}
				continue
			}
			if writeErr := a.handleInput(session, data); writeErr != nil {
				inputErrors <- writeErr
				return
			}
		}
	}()

	for {
		select {
		case output := <-events:
			if err := writeTerminalOutput(connection, output); err != nil {
				return
			}
		case <-done:
			for {
				select {
				case output := <-events:
					if err := writeTerminalOutput(connection, output); err != nil {
						return
					}
				default:
					_ = writeTerminalJSON(connection, map[string]any{"running": false, "type": "solomon-status"})
					_ = writeTerminalJSON(connection, map[string]any{"type": "solomon-exit"})
					return
				}
			}
		case <-inputErrors:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func terminalOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// CLI/TUI WebSocket clients do not send an Origin header. Browser
		// clients are checked below, so this does not weaken the browser path.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	// Wails serves the shared frontend from wails.localhost while its API
	// connection targets the daemon's loopback URL.
	originHost := strings.ToLower(parsed.Hostname())
	if originHost == "wails.localhost" {
		return true
	}
	requestHost := strings.ToLower(requestHostname(r.Host))
	if originHost == "localhost" && (requestHost == "localhost" || isLoopbackHost(requestHost)) {
		return true
	}
	if originIP := net.ParseIP(originHost); originIP != nil && originIP.IsLoopback() && isLoopbackHost(requestHost) {
		return true
	}
	return false
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func requestHostname(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(hostport, "[]")
}

func (a *terminalAPI) handleInput(session *terminalSession, data []byte) error {
	var control terminalControlMessage
	if len(data) > 0 && data[0] == '{' && json.Unmarshal(data, &control) == nil {
		switch control.Type {
		case "resize":
			return session.Resize(control.Cols, control.Rows)
		case "close":
			return errors.New("terminal session closed by client")
		}
	}
	return session.Write(data)
}

func writeTerminalJSON(connection *websocket.Conn, value any) error {
	return connection.WriteJSON(value)
}

func writeTerminalOutput(connection *websocket.Conn, output terminalOutput) error {
	return writeTerminalJSON(connection, map[string]any{
		"data": base64.StdEncoding.EncodeToString(output.Data),
		"seq":  output.Seq,
		"type": "solomon-output",
	})
}

func parseIntQuery(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseUintQuery(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}
