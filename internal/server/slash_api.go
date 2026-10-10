package server

import (
	"bytes"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func (a *chatAPI) handleProjectSlashCommands(w http.ResponseWriter, r *http.Request, projectID, root string) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	cfg, err := config.LoadOptional()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	descriptions := commands.SlashBuiltinDescriptions(cfg)
	query := strings.ToLower(strings.TrimPrefix(r.URL.Query().Get("query"), "/"))
	result := make([]map[string]string, 0)
	for _, name := range replcomplete.SlashCommandNames(replcomplete.ReplCompleteEnv{Cfg: cfg, ProjHex: projectID, ProjRoot: root}) {
		if !commands.GUISlashAvailable(name) || !strings.HasPrefix(name, query) {
			continue
		}
		description := descriptions[name]
		if description == "" {
			description = "Run installed skill"
		}
		result = append(result, map[string]string{"tag": "/" + name, "description": description})
	}
	writeJSON(w, http.StatusOK, result)
}

var slashANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func (a *chatAPI) handleSendSlashCommand(w http.ResponseWriter, r *http.Request, projectID, root, chatID string, sess *chatstore.Session, content string) {
	cfg, prov, err := loadChatRuntimeConfig()
	if err != nil {
		// Informational commands also work before a model is configured.
		cfg, err = config.LoadOptional()
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, err)
			return
		}
	}
	key := projectID + "\x00" + chatID
	run, ok := a.beginRun(key)
	if !ok {
		writeAPIError(w, http.StatusConflict, errors.New("chat is already running"))
		return
	}
	run.Emit(apiEvent("chat_start", map[string]any{"chat": apiChatFromSession(projectID, sess, run), "user": map[string]string{"role": "user", "content": content}}))
	go func() {
		defer a.finishRun(key, run)
		rt := agentruntime.NewRuntime(nil, cfg, prov, projectID, root, sess)
		defer rt.Close()
		var output bytes.Buffer
		rt.Out = &output
		rt.EventSink = run
		rt.InitMCP(run.ctx)
		before := len(sess.Messages)
		startedAt := time.Now().UTC()
		commandErr := rt.RunGUISlash(run.ctx, content)
		text := strings.TrimSpace(slashANSI.ReplaceAllString(output.String(), ""))
		if commandErr != nil {
			if text != "" {
				text += "\n\n"
			}
			text += commandErr.Error()
		}
		if len(sess.Messages) == before {
			sess.Messages = append(sess.Messages, chatstore.Message{Role: "user", Content: content, CreatedAt: time.Now().UTC()})
		}
		if text != "" {
			sess.Messages = append(sess.Messages, chatstore.Message{Role: "assistant", Content: text, CreatedAt: time.Now().UTC()})
		}
		sess.LastUserMessageAt = startedAt
		sess.LastMessageAt = time.Now().UTC()
		if err := chatstore.WriteSession(projectID, sess); err != nil {
			run.Emit(apiEvent("error", map[string]any{"error": err.Error()}))
		}
		run.Emit(apiEvent("chat_snapshot", map[string]any{"chat": apiChatFromSession(projectID, sess, nil)}))
		if commandErr != nil {
			run.Emit(apiEvent("error", map[string]any{"error": commandErr.Error()}))
		}
	}()
	a.streamChatRun(w, r, run, 0)
}
