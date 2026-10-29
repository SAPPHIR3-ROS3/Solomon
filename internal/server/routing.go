package server

import (
	"encoding/json"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *chatAPI) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data, err := a.loadProjectSidebarData()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	var req createProjectRequest
	if err := decodeJSONBody(w, r, &req, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, id, err := resolveAndRegisterProject(req.Path)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	info, err := a.projectInfo(root, id)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": info})
}

func (a *chatAPI) handleFastMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var req fastModeRequest
	if err := decodeJSONBody(w, r, &req, 1024); err != nil || req.FastMode == nil {
		if err == nil {
			err = errors.New("fastMode must be a boolean")
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := config.UpdateFastMode(*req.FastMode); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"fastMode": *req.FastMode})
}

func (a *chatAPI) handleProjectRoute(w http.ResponseWriter, r *http.Request) {
	parts := splitProjectRoute(r.URL.Path)
	if len(parts) == 0 {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	projectID, err := decodePathPart(parts[0])
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := projectRootForID(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	if len(parts) == 2 && parts[1] == "at-mentions" {
		a.handleProjectAtMentions(w, r, root)
		return
	}
	if len(parts) == 2 && parts[1] == "commands" {
		a.handleProjectCommand(w, r, projectID, root)
		return
	}
	if len(parts) == 2 && parts[1] == "slash-commands" {
		a.handleProjectSlashCommands(w, r, projectID, root)
		return
	}

	if len(parts) == 2 && parts[1] == "chats" {
		a.handleChatCollection(w, r, projectID)
		return
	}
	if len(parts) == 3 && parts[1] == "subchats" {
		subchatID, decodeErr := decodePathPart(parts[2])
		if decodeErr != nil || !safeChatID(subchatID) {
			writeAPIError(w, http.StatusBadRequest, errors.New("invalid subchat id"))
			return
		}
		a.handleSubchat(w, r, projectID, subchatID)
		return
	}
	if len(parts) < 3 || parts[1] != "chats" {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	chatID, err := decodePathPart(parts[2])
	if err != nil || !safeChatID(chatID) {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid chat id"))
		return
	}
	if len(parts) == 3 {
		a.handleChat(w, r, projectID, chatID)
		return
	}

	switch parts[3] {
	case "messages":
		if len(parts) == 4 && r.Method == http.MethodPost {
			a.handleSendChatMessage(w, r, projectID, root, chatID)
			return
		}
		if len(parts) == 5 && r.Method == http.MethodDelete {
			messageID, decodeErr := decodePathPart(parts[4])
			if decodeErr != nil {
				writeAPIError(w, http.StatusBadRequest, decodeErr)
				return
			}
			a.handleDeleteChatMessage(w, r, projectID, chatID, messageID)
			return
		}
	case "events":
		if len(parts) == 4 && r.Method == http.MethodGet {
			a.handleChatEvents(w, r, projectID, chatID)
			return
		}
	case "stop":
		if len(parts) == 4 && r.Method == http.MethodPost {
			a.handleStopChat(w, r, projectID, chatID)
			return
		}
	case "images":
		if len(parts) == 5 && r.Method == http.MethodGet {
			seq, parseErr := strconv.Atoi(parts[4])
			if parseErr != nil || seq < 0 {
				writeAPIError(w, http.StatusBadRequest, errors.New("invalid image sequence"))
				return
			}
			a.handleChatImage(w, r, projectID, chatID, seq)
			return
		}
	}

	writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
}

// handlesProjectRoute reports whether the daemon owns this project route.
// The GUI development server owns the auxiliary project surfaces (files,
// research and Git/worktrees), while chats and at-mentions must always use the
// daemon runtime. Keeping this boundary explicit prevents the broad project
// route from swallowing the existing GUI handlers with a 404.
func (a *chatAPI) handlesProjectRoute(path string) bool {
	parts := splitProjectRoute(path)
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "at-mentions", "slash-commands", "commands", "chats", "subchats":
		return true
	default:
		return false
	}
}

func (a *chatAPI) handleProjectAtMentions(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	entries, err := a.atMentionIndex.Get(r.Context(), root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	query := r.URL.Query().Get("query")
	matches := atmention.MatchQuery(query, entries, atmention.MaxPickerResults)
	if strings.TrimSpace(query) == "" {
		matches = atmention.InitialPickerEntries(entries, atmention.MaxPickerResults)
	}
	result := make([]apiAtMentionSuggestion, 0, len(matches))
	for _, entry := range matches {
		result = append(result, apiAtMentionSuggestion{
			IsDirectory: entry.IsDir,
			Path:        entry.RelPath,
			Tag:         "@" + atmention.ShortTag(entry.RelPath, entries),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func parseStartingAfter(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("starting_after"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("after"))
	}
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("starting_after must be a non-negative integer")
	}
	return value, nil
}

func (a *chatAPI) loadProjectSidebarData() (apiProjectSidebar, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return apiProjectSidebar{}, err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return apiProjectSidebar{}, err
	}
	projects := make([]apiProject, 0, len(projectMap))
	for root, id := range projectMap {
		if !safeProjectID(id) {
			continue
		}
		info, infoErr := a.projectInfo(root, id)
		if infoErr != nil {
			return apiProjectSidebar{}, infoErr
		}
		projects = append(projects, info)
	}
	sort.SliceStable(projects, func(i, j int) bool {
		left := projectLastActivity(projects[i])
		right := projectLastActivity(projects[j])
		return left.After(right) || (left.Equal(right) && projects[i].Name < projects[j].Name)
	})
	cfg, _ := config.LoadOptional()
	reasoning := "none"
	userName := ""
	if cfg != nil {
		userName = strings.TrimSpace(cfg.UserName)
		if normalized, parseErr := config.ParseReasoningEffortToken(cfg.ReasoningEffort); parseErr == nil {
			reasoning = normalized
		}
	}
	fastMode := true
	if cfg != nil {
		fastMode = cfg.EffectiveFastMode()
	}
	return apiProjectSidebar{FastMode: fastMode, Projects: projects, ReasoningEffort: reasoning, UserName: userName}, nil
}

func (a *chatAPI) projectInfo(root, id string) (apiProject, error) {
	sessions, err := chatstore.ListRecent(id, 10000)
	if err != nil {
		return apiProject{}, err
	}
	chats := make([]apiChatSummary, 0, len(sessions))
	for _, sess := range sessions {
		if sess == nil {
			continue
		}
		last := sidebarChatSentTime(sess)
		title := strings.TrimSpace(sess.Title)
		if title == "" {
			title = "Untitled chat"
		}
		chats = append(chats, apiChatSummary{ID: sess.ID, LastMessageAt: last, Title: title})
	}
	sort.SliceStable(chats, func(i, j int) bool {
		return chats[i].LastMessageAt.After(chats[j].LastMessageAt)
	})
	var user, reasoning, response int64
	if _, u, r, resp, statsErr := chatstore.ProjectWelcomeStats(id); statsErr == nil {
		user, reasoning, response = u, r, resp
	}
	name := filepath.Base(filepath.Clean(root))
	if home, homeErr := os.UserHomeDir(); homeErr == nil && filepath.Clean(root) == filepath.Clean(home) {
		name = "Home"
	}
	return apiProject{
		Chats:      chats,
		ID:         id,
		Name:       name,
		Path:       root,
		ChatCount:  len(chats),
		TokenStats: apiTokenStats{User: user, Reasoning: reasoning, Response: response, Total: user + reasoning + response},
	}, nil
}

// Sidebar order follows user sends, never assistant output or run completion.
func sidebarChatSentTime(sess *chatstore.Session) time.Time {
	if !sess.LastUserMessageAt.IsZero() {
		return sess.LastUserMessageAt
	}
	latest := sess.CreatedAt
	for _, message := range sess.Messages {
		if message.Role == "user" && message.CreatedAt.After(latest) {
			latest = message.CreatedAt
		}
	}
	return latest
}

func projectLastActivity(p apiProject) time.Time {
	var latest time.Time
	for _, chat := range p.Chats {
		if chat.LastMessageAt.After(latest) {
			latest = chat.LastMessageAt
		}
	}
	return latest
}

func resolveAndRegisterProject(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	if value == "" || value == "~" || value == "~/" {
		value = home
	} else if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(value, "~/")))
	} else if !filepath.IsAbs(value) {
		value = filepath.Join(home, filepath.FromSlash(value))
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", errors.New("project path is not a directory")
	}
	return project.Resolve(value)
}

func projectRootForID(id string) (string, error) {
	if !safeProjectID(id) {
		return "", os.ErrNotExist
	}
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return "", err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return "", err
	}
	for root, projectID := range projectMap {
		if projectID == id {
			return root, nil
		}
	}
	return "", os.ErrNotExist
}

func splitProjectRoute(path string) []string {
	trimmed := strings.TrimPrefix(path, chatAPIPath+"/")
	if trimmed == path || trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func decodePathPart(value string) (string, error) {
	return url.PathUnescape(value)
}

func safeProjectID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func safeChatID(value string) bool {
	return value != "" && len(value) <= 256 && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`)
}

func parseMessageID(value string) (int, error) {
	if !strings.HasPrefix(value, "m-") {
		return 0, errors.New("invalid message id")
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(value, "m-"))
	if err != nil || idx < 0 {
		return 0, errors.New("invalid message id")
	}
	return idx, nil
}

func errOrNotFound(err error) error {
	if err != nil {
		return err
	}
	return os.ErrNotExist
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	message := "request failed"
	if err != nil {
		message = err.Error()
	}
	logging.RecordNotification("error", message)
	writeJSON(w, status, map[string]string{"error": message})
}
