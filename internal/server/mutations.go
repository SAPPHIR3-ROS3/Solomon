package server

import (
	"errors"
	"fmt"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a *chatAPI) handleChatCollection(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var req createChatRequest
	if err := decodeJSONBody(w, r, &req, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	now := time.Now().UTC()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New chat"
	}
	chatID := chatstore.ChatIDHex(title, now)
	sess := &chatstore.Session{
		ID:             chatID,
		Title:          title,
		CreatedAt:      now,
		LastMessageAt:  now,
		CheckpointLast: -1,
		CheckpointCP0:  true,
	}
	if err := chatstore.WriteSession(projectID, sess); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, apiChatFromSession(projectID, sess, nil))
}

func (a *chatAPI) handleChat(w http.ResponseWriter, r *http.Request, projectID, chatID string) {
	if r.Method == http.MethodPatch {
		a.handleRenameChat(w, r, projectID, chatID)
		return
	}
	if r.Method == http.MethodDelete {
		a.handleDeleteChat(w, r, projectID, chatID)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, apiChatFromSession(projectID, sess, a.getRun(projectID+"\x00"+chatID)))
}

func (a *chatAPI) handleRenameChat(w http.ResponseWriter, r *http.Request, projectID, chatID string) {
	if a.runActive(projectID + "\x00" + chatID) {
		writeAPIError(w, http.StatusConflict, errors.New("chat is running"))
		return
	}
	var req renameChatRequest
	if err := decodeJSONBody(w, r, &req, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("chat title is required"))
		return
	}
	if len([]rune(title)) > maxChatTitleLength {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("chat title must be at most %d characters", maxChatTitleLength))
		return
	}
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, err)
		return
	}
	sess.ID = chatID
	sess.Title = title
	if err := chatstore.WriteSession(projectID, sess); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiChatFromSession(projectID, sess, nil))
}

func (a *chatAPI) handleDeleteChat(w http.ResponseWriter, _ *http.Request, projectID, chatID string) {
	if a.runActive(projectID + "\x00" + chatID) {
		writeAPIError(w, http.StatusConflict, errors.New("chat is running"))
		return
	}
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, err)
		return
	}
	if err := chatstore.RemoveSessionPath(projectID, chatID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if a.browsers != nil {
		_ = a.browsers.Remove(projectID, chatID)
	}
	for _, filePath := range sess.ImageFiles {
		if isSafeProjectImagePath(projectID, filePath) {
			_ = os.Remove(filePath)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *chatAPI) handleDeleteChatMessage(w http.ResponseWriter, _ *http.Request, projectID, chatID, messageID string) {
	idx, err := parseMessageID(messageID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if a.runActive(projectID + "\x00" + chatID) {
		writeAPIError(w, http.StatusConflict, errors.New("chat is running"))
		return
	}
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, err)
		return
	}
	if idx < 0 || idx >= len(sess.Messages) || sess.Messages[idx].Role != "user" {
		writeAPIError(w, http.StatusNotFound, errors.New("user message not found"))
		return
	}
	end := idx + 1
	for end < len(sess.Messages) && sess.Messages[end].Role != "user" {
		end++
	}
	sess.Messages = append(sess.Messages[:idx], sess.Messages[end:]...)
	if len(sess.Messages) == 0 {
		sess.LastUserMessageAt = time.Time{}
	}
	sess.LastMessageAt = time.Now().UTC()
	if err := chatstore.WriteSession(projectID, sess); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiChatFromSession(projectID, sess, nil))
}

func (a *chatAPI) handleStopChat(w http.ResponseWriter, _ *http.Request, projectID, chatID string) {
	if !a.cancelRun(projectID + "\x00" + chatID) {
		writeAPIError(w, http.StatusNotFound, errors.New("chat is not running"))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"stopping": true})
}

func (a *chatAPI) handleSubchat(w http.ResponseWriter, r *http.Request, projectID, subchatID string) {
	if r.Method == http.MethodGet {
		sess, err := chatstore.FindSubSessionByID(projectID, subchatID)
		if err != nil || (sess != nil && sess.ProjectHex != "" && sess.ProjectHex != projectID) {
			status := http.StatusInternalServerError
			if errors.Is(err, os.ErrNotExist) || sess == nil {
				status = http.StatusNotFound
			}
			writeAPIError(w, status, errOrNotFound(err))
			return
		}
		writeJSON(w, http.StatusOK, apiChatFromSubSession(projectID, sess))
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var req controlSubchatRequest
	if err := decodeJSONBody(w, r, &req, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "stop" && action != "cancel" && action != "resume" {
		writeAPIError(w, http.StatusBadRequest, errors.New("unknown subchat control action"))
		return
	}
	var controlErr error
	if action == "resume" {
		root, rootErr := projectRootForID(projectID)
		if rootErr != nil {
			writeAPIError(w, http.StatusNotFound, rootErr)
			return
		}
		cfg, prov, cfgErr := loadChatRuntimeConfig()
		if cfgErr != nil {
			writeAPIError(w, http.StatusServiceUnavailable, cfgErr)
			return
		}
		rt := agentruntime.NewRuntime(nil, cfg, prov, projectID, root, &chatstore.Session{ID: "subchat-control"})
		rt.Out = io.Discard
		controlErr = rt.ControlSubagent(subchatID, action)
		_ = rt.Close()
	} else {
		controlErr = (&agentruntime.Runtime{ProjHex: projectID, Out: io.Discard}).ControlSubagent(subchatID, action)
	}
	if controlErr != nil {
		writeAPIError(w, http.StatusConflict, controlErr)
		return
	}
	if sess, readErr := chatstore.FindSubSessionByID(projectID, subchatID); readErr == nil {
		writeJSON(w, http.StatusOK, apiChatFromSubSession(projectID, sess))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}
