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
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
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

func (a *chatAPI) handleSendSlashCommand(w http.ResponseWriter, r *http.Request, projectID, root, chatID string, sess *chatstore.Session, content, surface string) {
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
	run.Emit(apiEvent("chat_start", map[string]any{"chat": apiChatFromSession(projectID, sess, run)}))
	go func() {
		defer a.finishRun(key, run)
		rt := agentruntime.NewCommandRuntime(run.ctx, cfg, prov, projectID, root, sess, content)
		defer rt.Close()
		var output bytes.Buffer
		rt.Out = &output
		rt.EventSink = run
		rt.Surface = surface
		before := len(sess.Messages)
		startedAt := time.Now().UTC()
		commandErr := rt.RunGUISlash(run.ctx, content)
		text := strings.TrimSpace(slashANSI.ReplaceAllString(output.String(), ""))
		if commandErr != nil {
			text += "\n" + commandErr.Error()
		}
		commandRecorded := false
		for index := before; index < len(sess.Messages); index++ {
			if sess.Messages[index].Role == "user" && sess.Messages[index].Content == content {
				sess.Messages[index].GUIHidden = true
				commandRecorded = true
			}
		}
		if !commandRecorded {
			sess.Messages = append(sess.Messages, chatstore.Message{GUIHidden: true, Role: "user", Content: content, CreatedAt: time.Now().UTC()})
		}
		if text != "" {
			sess.Messages = append(sess.Messages, chatstore.Message{GUIHidden: true, Role: "assistant", Content: text, CreatedAt: time.Now().UTC()})
		}
		sess.LastUserMessageAt = startedAt
		sess.LastMessageAt = time.Now().UTC()
		if err := chatstore.WriteSession(projectID, sess); err != nil {
			run.Emit(apiEvent("error", map[string]any{"error": err.Error()}))
		}
		if commandErr == nil {
			for _, notice := range slashCommandNotifications(output.String()) {
				run.Emit(apiEvent("notification", map[string]any{"message": notice["message"], "level": notice["level"]}))
			}
		}
		run.Emit(apiEvent("chat_snapshot", map[string]any{"chat": apiChatFromSession(projectID, sess, nil)}))
		if commandErr != nil {
			run.Emit(apiEvent("error", map[string]any{"error": commandErr.Error()}))
		}
	}()
	a.streamChatRun(w, r, run, 0)
}

// System blocks are already recorded by WriteSystem; only command data belongs in chat.
func slashChatOutput(output string) string {
	text := slashANSI.ReplaceAllString(output, "")
	parts := strings.Split(text, "===SYSTEM===")
	var result strings.Builder
	for index, part := range parts {
		if index%2 == 0 {
			result.WriteString(part)
		}
	}
	return strings.TrimSpace(result.String())
}

// Return command feedback with the HTTP/SSE response, without waiting for log polling.
func slashCommandNotifications(output string) []map[string]string {
	text := slashANSI.ReplaceAllString(output, "")
	notices := make([]map[string]string, 0)
	for index, part := range strings.Split(text, "===SYSTEM===") {
		message := strings.TrimSpace(part)
		if message == "" {
			continue
		}
		// WriteSystem already records enclosed blocks; record plain command output here.
		if index%2 == 0 {
			logging.RecordNotification("system", message)
		}
		notices = append(notices, map[string]string{"level": "system", "message": message})
	}
	return notices
}
