package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/programming"
	"github.com/gorilla/websocket"
)

func languageServerHandler(service context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		root, err := registeredProjectRoot(r.URL.Query().Get("project"))
		if err != nil {
			writeAPIError(w, 404, err)
			return
		}
		if _, err = safeWorkspacePath(root, r.URL.Query().Get("path")); err != nil {
			writeAPIError(w, 400, err)
			return
		}
		languages, err := config.ReadProgrammingLanguages()
		if err != nil {
			writeAPIError(w, 500, err)
			return
		}
		id := r.URL.Query().Get("language")
		language, ok := languages[id]
		if !ok {
			writeAPIError(w, 404, errors.New("language has not been registered"))
			return
		}
		server := programming.Resolve(id, language)
		if !server.Available {
			writeAPIError(w, 409, errors.New(server.Reason))
			return
		}
		upgrader := websocket.Upgrader{CheckOrigin: terminalOriginAllowed}
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		connection.SetReadLimit(programming.MaxMessageSize)
		ctx, cancel := context.WithCancel(service)
		defer cancel()
		command := exec.CommandContext(ctx, server.Command[0], server.Command[1:]...)
		command.Dir = root
		command.Stderr = io.Discard
		input, err := command.StdinPipe()
		if err != nil {
			return
		}
		output, err := command.StdoutPipe()
		if err != nil {
			return
		}
		if err = command.Start(); err != nil {
			connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1011, "Language server could not start"), time.Now().Add(time.Second))
			return
		}
		defer func() { input.Close(); cancel(); command.Wait() }()
		var once sync.Once
		stop := func() { once.Do(func() { cancel(); connection.Close() }) }
		defer stop()
		go func() { <-ctx.Done(); stop() }()
		go func() {
			defer stop()
			reader := bufio.NewReader(output)
			for {
				data, err := programming.ReadMessage(reader)
				if err != nil {
					return
				}
				if !json.Valid(data) {
					return
				}
				connection.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if connection.WriteMessage(websocket.TextMessage, data) != nil {
					return
				}
			}
		}()
		for {
			kind, data, err := connection.ReadMessage()
			if err != nil {
				return
			}
			if kind != websocket.TextMessage || !json.Valid(data) {
				return
			}
			if pipe, ok := input.(interface{ SetWriteDeadline(time.Time) error }); ok {
				_ = pipe.SetWriteDeadline(time.Now().Add(15 * time.Second))
			}
			if programming.WriteMessage(input, data) != nil {
				return
			}
		}
	}
}
