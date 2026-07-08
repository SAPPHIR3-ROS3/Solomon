package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type createResponseBody struct {
	Model        string          `json:"model"`
	Conversation string          `json:"conversation"`
	Input        json.RawMessage `json:"input"`
	Stream       bool            `json:"stream"`
	Background   bool            `json:"background"`
	Metadata     map[string]any  `json:"metadata"`
	Instructions string          `json:"instructions"`
}

func (s *Server) handleCreateResponse(w http.ResponseWriter, r *http.Request) {
	if s.hub.TurnActive() {
		writeError(w, http.StatusConflict, "turn_active", "a turn is already in progress")
		return
	}
	var body createResponseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}
	text, err := extractInputText(body.Input)
	if err != nil || text == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "input is required")
		return
	}
	sess, err := s.loadSession(body.Conversation)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	lock, err := acquireSessionLock(s.projHex, conversationIDForSession(sess))
	if err != nil {
		writeError(w, http.StatusConflict, "session_locked", err.Error())
		return
	}

	responseID := newResponseID()
	if isSlashInput(text) {
		defer lock.Release()
		s.handleSlashResponse(w, r, responseID, body.Conversation, text, body.Metadata, body.Stream)
		return
	}
	if body.Background {
		go func() {
			defer lock.Release()
			s.runTurnBackground(responseID, body.Conversation, text, sess)
		}()
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     responseID,
			"object": "response",
			"status": "queued",
		})
		return
	}
	defer lock.Release()
	if body.Stream {
		s.runTurnStream(w, r, responseID, body.Conversation, text, sess)
		return
	}
	out, err := s.runTurnSync(r.Context(), responseID, body.Conversation, text, sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSlashResponse(w http.ResponseWriter, r *http.Request, responseID, conversation, line string, meta map[string]any, stream bool) {
	rt, err := s.ensureRuntime()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if conversation != "" {
		sess, err := chatstore.ReadSession(s.projHex, conversation)
		if err == nil {
			rt.Session = sess
		}
	}
	_ = metadataSlashPayload(meta)
	var buf bytes.Buffer
	deps := s.slashDeps(rt, &buf)
	if err := slash.Dispatch(deps, line); err != nil {
		writeError(w, http.StatusBadRequest, "slash_error", err.Error())
		return
	}
	out := collectSlashOutput(&buf)
	resp := slashOutputResponse(responseID, conversation, out)
	s.saveSlashResponse(responseID, conversation, out)
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_ = writeSSE(w, 1, "response.created", map[string]any{"response": resp})
		_ = writeSSE(w, 2, "response.completed", map[string]any{"response": resp})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) runTurnSync(ctx context.Context, responseID, conversation, prompt string, sess *chatstore.Session) (map[string]any, error) {
	buf := s.hub.BeginTurn(responseID, conversation, func() {})
	buf.append("response.created", map[string]any{
		"id": responseID, "object": "response", "status": "in_progress", "conversation": conversation,
	})
	final, err := s.executeTurn(ctx, responseID, conversation, prompt, sess, buf)
	status, output := finishTurnStatus(err, final)
	buf.append("response.completed", map[string]any{"id": responseID, "status": status, "output_text": output})
	s.hub.FinishTurn(status, output)
	if err != nil && status == "failed" {
		return nil, err
	}
	return map[string]any{
		"id":           responseID,
		"object":       "response",
		"status":       status,
		"conversation": conversation,
		"output_text":  output,
	}, nil
}

func finishTurnStatus(err error, final string) (status, output string) {
	output = final
	if err == nil {
		return "completed", output
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", output
	}
	return "failed", output
}

func (s *Server) runTurnStream(w http.ResponseWriter, r *http.Request, responseID, conversation, prompt string, sess *chatstore.Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "server_error", "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	buf := s.hub.BeginTurn(responseID, conversation, cancel)
	buf.append("response.created", map[string]any{
		"id": responseID, "object": "response", "status": "in_progress", "conversation": conversation,
	})
	flusher.Flush()
	done := make(chan struct{})
	go func() {
		defer close(done)
		final, err := s.executeTurn(ctx, responseID, conversation, prompt, sess, buf)
		status, output := finishTurnStatus(err, final)
		if err != nil && status == "failed" {
			buf.append("error", map[string]any{"message": err.Error()})
		}
		buf.append("response.completed", map[string]any{"id": responseID, "status": status, "output_text": output})
		s.hub.FinishTurn(status, output)
	}()
	_ = streamFromHub(ctx, w, s.hub, buf, 0)
	<-done
	flusher.Flush()
}

func (s *Server) runTurnBackground(responseID, conversation, prompt string, sess *chatstore.Session) {
	ctx, cancel := context.WithCancel(context.Background())
	buf := s.hub.BeginTurn(responseID, conversation, cancel)
	buf.append("response.created", map[string]any{
		"id": responseID, "object": "response", "status": "in_progress", "conversation": conversation,
	})
	final, err := s.executeTurn(ctx, responseID, conversation, prompt, sess, buf)
	status, output := finishTurnStatus(err, final)
	if err != nil && status == "failed" {
		buf.append("error", map[string]any{"message": err.Error()})
	}
	buf.append("response.completed", map[string]any{"id": responseID, "status": status, "output_text": output})
	s.hub.FinishTurn(status, output)
}

func (s *Server) saveSlashResponse(responseID, conversation, text string) {
	if s.hub.store == nil {
		return
	}
	created := map[string]any{"id": responseID, "object": "response", "status": "completed", "conversation": conversation, "output_text": text}
	completed := map[string]any{"id": responseID, "status": "completed", "output_text": text}
	rec := persistedResponse{
		ID:           responseID,
		Object:       "response",
		Conversation: conversation,
		Status:       "completed",
		OutputText:   text,
		Events: []streamEvent{
			{ID: 1, Type: "response.created", Data: map[string]any{"response": created}},
			{ID: 2, Type: "response.completed", Data: map[string]any{"response": completed}},
		},
		CompletedAt: time.Now().UTC(),
	}
	_ = s.hub.store.Put(rec)
}

func (s *Server) executeTurn(ctx context.Context, responseID, conversation, prompt string, sess *chatstore.Session, buf *eventBuffer) (string, error) {
	rt := agentruntime.NewRuntime(nil, s.cfg, s.prov, s.projHex, s.projRoot, sess)
	rt.InitMCP(context.Background())
	defer rt.Close()
	if buf != nil {
		rt.EventSink = newHubSink(s.hub, buf)
	}
	if err := rt.RunPromptOnce(ctx, prompt); err != nil {
		return "", err
	}
	if conversation == "" {
		conversation = sess.ID
	}
	_ = responseID
	return lastAssistantText(sess), nil
}

func lastAssistantText(sess *chatstore.Session) string {
	if sess == nil {
		return ""
	}
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == "assistant" {
			return sess.Messages[i].Content
		}
	}
	return ""
}

func (s *Server) handleGetResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if r.URL.Query().Get("stream") == "true" {
		s.handleStreamResponse(w, r, id)
		return
	}
	if s.hub.ActiveResponseID() == id {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     id,
			"object": "response",
			"status": "in_progress",
		})
		return
	}
	if rec, ok := s.hub.GetStored(id); ok {
		writeJSON(w, http.StatusOK, s.hub.store.Snapshot(*rec))
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "response not found")
}

func (s *Server) handleStreamResponse(w http.ResponseWriter, r *http.Request, id string) {
	after := 0
	if v := strings.TrimSpace(r.URL.Query().Get("starting_after")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			after = n
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ctx := r.Context()
	if s.hub.ActiveResponseID() == id {
		buf := s.hub.ActiveBuffer()
		if buf == nil {
			writeError(w, http.StatusNotFound, "not_found", "response not found")
			return
		}
		if err := streamFromHub(ctx, w, s.hub, buf, after); err != nil && ctx.Err() == nil {
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		}
		return
	}
	if rec, ok := s.hub.GetStored(id); ok {
		if err := replayEvents(w, rec.Events, after); err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		}
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "response not found or completed")
}

func (s *Server) handleCancelResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.hub.ActiveResponseID() != id {
		writeError(w, http.StatusNotFound, "not_found", "response not active")
		return
	}
	if !s.hub.CancelActive() {
		writeError(w, http.StatusConflict, "cancel_failed", "unable to cancel")
		return
	}
	done := s.hub.WaitTurnDone(id)
	select {
	case <-done:
	case <-time.After(30 * time.Second):
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "cancelled"})
}
