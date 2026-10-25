package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

// Home commands have their own transcript, without creating a chat session.
func (a *chatAPI) handleProjectCommand(w http.ResponseWriter, r *http.Request, projectID, root string) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var req sendChatRequest
	if err := decodeJSONBody(w, r, &req, maxChatRequestBytes); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if !strings.HasPrefix(content, "/") || len(req.Images) > 0 || len(req.Clips) > 0 {
		writeAPIError(w, http.StatusBadRequest, errors.New("a slash command without attachments is required"))
		return
	}
	// Skills run an agent turn, so the GUI sends them to a new chat where the turn is visible.
	if projectID != "" && slash.RunsSkill(content, projectID, root) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "open_chat": true})
		return
	}
	cfg, prov, err := loadChatRuntimeConfig()
	if err != nil {
		cfg, err = config.LoadOptional()
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, err)
			return
		}
	}
	key := projectID + "\x00commands"
	run, ok := a.beginRun(key)
	if !ok {
		writeAPIError(w, http.StatusConflict, errors.New("a project command is already running"))
		return
	}
	defer a.finishRun(key, run)
	projectDir, err := paths.ProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if err = os.MkdirAll(projectDir, 0700); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	transcript, err := os.OpenFile(filepath.Join(projectDir, "commands.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	defer transcript.Close()
	encoder := json.NewEncoder(transcript)
	if err := encoder.Encode(chatstore.Message{Role: "user", Content: content, CreatedAt: time.Now().UTC(), GUIHidden: true}); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	sess := &chatstore.Session{CreatedAt: time.Now().UTC()}
	rt := agentruntime.NewCommandRuntime(run.ctx, cfg, prov, projectID, root, sess, content)
	defer rt.Close()
	rt.EphemeralSession = true
	var output bytes.Buffer
	rt.Out = &output
	rt.EventSink = run
	rt.Surface = clientSurface(r, req.Surface)
	commandErr := rt.RunGUISlash(run.ctx, content)
	text := strings.TrimSpace(slashANSI.ReplaceAllString(output.String(), ""))
	if commandErr != nil {
		text += "\n" + commandErr.Error()
	}
	if text != "" {
		if err := encoder.Encode(chatstore.Message{Role: "assistant", Content: strings.TrimSpace(text), CreatedAt: time.Now().UTC(), GUIHidden: true}); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if commandErr != nil {
		writeAPIError(w, http.StatusBadRequest, commandErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "notifications": slashCommandNotifications(output.String())})
}
