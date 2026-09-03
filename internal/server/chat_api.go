package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const (
	chatAPIPath         = "/__solomon/projects"
	maxChatRequestBytes = 32 << 20
	chatTitleTimeout    = 30 * time.Second
)

var imageTagPattern = regexp.MustCompile(`\[img-(\d+)\]`)

type chatAPI struct {
	daemonCtx      context.Context
	activeMu       sync.Mutex
	active         map[string]*chatRun
	atMentionIndex *atmention.IndexCache
}

type chatRun struct {
	ctx            context.Context
	startedAt      time.Time
	cancel         context.CancelFunc
	done           chan struct{}
	finishOnce     sync.Once
	stateMu        sync.Mutex
	finished       bool
	events         []cievents.Event
	subscribers    map[uint64]chatRunSubscriber
	nextSubscriber uint64
}

type chatRunSubscriber struct {
	ctx    context.Context
	events chan cievents.Event
}

func newChatAPI(daemonCtx context.Context) *chatAPI {
	return &chatAPI{
		daemonCtx:      daemonCtx,
		active:         make(map[string]*chatRun),
		atMentionIndex: atmention.NewIndexCache(),
	}
}

func newChatRun(parent context.Context) *chatRun {
	ctx, cancel := context.WithCancel(parent)
	return &chatRun{
		ctx:         ctx,
		startedAt:   time.Now().UTC(),
		cancel:      cancel,
		done:        make(chan struct{}),
		events:      make([]cievents.Event, 0, 64),
		subscribers: make(map[uint64]chatRunSubscriber),
	}
}

func (r *chatRun) Emit(ev cievents.Event) {
	r.stateMu.Lock()
	if r.finished {
		r.stateMu.Unlock()
		return
	}
	value := make(cievents.Event, len(ev)+1)
	for key, item := range ev {
		value[key] = item
	}
	value["seq"] = int64(len(r.events) + 1)
	r.events = append(r.events, value)
	type subscriberEntry struct {
		id  uint64
		sub chatRunSubscriber
	}
	subscribers := make([]subscriberEntry, 0, len(r.subscribers))
	for id, subscriber := range r.subscribers {
		subscribers = append(subscribers, subscriberEntry{id: id, sub: subscriber})
	}
	r.stateMu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber.sub.events <- value:
		case <-subscriber.sub.ctx.Done():
			r.unsubscribe(subscriber.id)
		}
	}
}

func (r *chatRun) StreamMode() bool { return true }

func (r *chatRun) Events() []cievents.Event { return nil }

func (r *chatRun) FlushReport(cievents.ReportMeta, int, string, string, any) error { return nil }

func (r *chatRun) subscribe(ctx context.Context, startingAfter int64) ([]cievents.Event, <-chan cievents.Event, <-chan struct{}, func()) {
	subscriber := chatRunSubscriber{ctx: ctx, events: make(chan cievents.Event, 256)}
	r.stateMu.Lock()
	replay := make([]cievents.Event, 0, len(r.events))
	for _, ev := range r.events {
		seq, _ := ev["seq"].(int64)
		if seq > startingAfter {
			replay = append(replay, ev)
		}
	}
	id := r.nextSubscriber
	r.nextSubscriber++
	if !r.finished {
		r.subscribers[id] = subscriber
	}
	done := r.done
	r.stateMu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() { r.unsubscribe(id) })
	}
	return replay, subscriber.events, done, unsubscribe
}

func (r *chatRun) unsubscribe(id uint64) {
	r.stateMu.Lock()
	delete(r.subscribers, id)
	r.stateMu.Unlock()
}

func (r *chatRun) finish() {
	r.finishOnce.Do(func() {
		r.stateMu.Lock()
		r.finished = true
		close(r.done)
		r.stateMu.Unlock()
	})
}

type apiProject struct {
	Chats      []apiChatSummary `json:"chats"`
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	ChatCount  int              `json:"chatCount"`
	TokenStats apiTokenStats    `json:"tokenStats"`
}

type apiChatSummary struct {
	ID            string    `json:"id"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	Title         string    `json:"title"`
}

type apiTokenStats struct {
	User      int64 `json:"user"`
	Reasoning int64 `json:"reasoning"`
	Response  int64 `json:"response"`
	Total     int64 `json:"total"`
}

type apiAtMentionSuggestion struct {
	IsDirectory bool   `json:"isDirectory"`
	Path        string `json:"path"`
	Tag         string `json:"tag"`
}

type apiProjectSidebar struct {
	FastMode        bool         `json:"fastMode"`
	Projects        []apiProject `json:"projects"`
	ReasoningEffort string       `json:"reasoningEffort"`
	UserName        string       `json:"userName"`
}

type fastModeRequest struct {
	FastMode *bool `json:"fastMode"`
}

type apiChat struct {
	CreatedAt    string       `json:"createdAt,omitempty"`
	ID           string       `json:"id"`
	Messages     []apiMessage `json:"messages"`
	Mode         string       `json:"mode,omitempty"`
	ProjectID    string       `json:"projectID,omitempty"`
	Status       string       `json:"status,omitempty"`
	RunStartedAt string       `json:"runStartedAt,omitempty"`
	Title        string       `json:"title"`
}

type apiMessage struct {
	CreatedAt        string               `json:"createdAt,omitempty"`
	CheckpointBranch string               `json:"checkpointBranch,omitempty"`
	CheckpointSeq    *int                 `json:"checkpointSeq,omitempty"`
	Content          string               `json:"content"`
	ID               string               `json:"id"`
	Images           []apiImage           `json:"images,omitempty"`
	Kind             string               `json:"kind,omitempty"`
	Reasoning        string               `json:"reasoning,omitempty"`
	RetainedMessages []apiRetainedMessage `json:"retainedMessages,omitempty"`
	Role             string               `json:"role"`
	Stats            *apiStats            `json:"stats,omitempty"`
	Status           string               `json:"status,omitempty"`
	Summary          string               `json:"summary,omitempty"`
	ThoughtFor       float64              `json:"thoughtFor,omitempty"`
	ToolCalls        []apiToolCall        `json:"toolCalls,omitempty"`
	WorkedFor        float64              `json:"workedFor,omitempty"`
}

type apiRetainedMessage struct {
	Content string     `json:"content"`
	Images  []apiImage `json:"images,omitempty"`
	Role    string     `json:"role"`
}

type apiImage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type apiStats struct {
	ContextTokens         int64   `json:"contextTokens"`
	OutputTokensPerSecond float64 `json:"outputTokensPerSecond"`
	PromptTokensPerSecond float64 `json:"promptTokensPerSecond"`
	ReasoningTokens       int64   `json:"reasoningTokens"`
	ResponseTokens        int64   `json:"responseTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	TTFTSeconds           float64 `json:"ttftSeconds"`
	UserTokens            int64   `json:"userTokens"`
}

type apiToolCall struct {
	CheckpointBranch string             `json:"checkpointBranch,omitempty"`
	CheckpointSeq    *int               `json:"checkpointSeq,omitempty"`
	DefaultOpen      bool               `json:"defaultOpen,omitempty"`
	Delete           bool               `json:"delete,omitempty"`
	Full             bool               `json:"full,omitempty"`
	ID               string             `json:"id"`
	Input            string             `json:"input,omitempty"`
	Intent           string             `json:"intent"`
	Mode             string             `json:"mode,omitempty"`
	Name             string             `json:"name"`
	NewString        string             `json:"newString,omitempty"`
	OldString        string             `json:"oldString,omitempty"`
	Parameters       []apiToolParameter `json:"parameters,omitempty"`
	RenameTo         string             `json:"renameTo,omitempty"`
	Result           map[string]any     `json:"result,omitempty"`
	Status           string             `json:"status,omitempty"`
	Sync             bool               `json:"sync,omitempty"`
}

type apiToolParameter struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type createProjectRequest struct {
	Path string `json:"path"`
}

type createChatRequest struct {
	Title string `json:"title,omitempty"`
}

type sendChatRequest struct {
	Content string          `json:"content"`
	Images  []incomingImage `json:"images,omitempty"`
}

type incomingImage struct {
	Data string `json:"data,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type controlSubchatRequest struct {
	Action string `json:"action"`
}

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
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	cfg.FastMode = req.FastMode
	if err := config.Save(cfg); err != nil {
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
	case "at-mentions", "chats", "subchats":
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

func (a *chatAPI) handleSendChatMessage(w http.ResponseWriter, r *http.Request, projectID, projectRoot, chatID string) {
	var req sendChatRequest
	if err := decodeJSONBody(w, r, &req, maxChatRequestBytes); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" && len(req.Images) == 0 {
		writeAPIError(w, http.StatusBadRequest, errors.New("message is empty"))
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
	generateTitle := needsChatTitleGeneration(sess)
	titleInput := chatTitleInput(req.Content, len(req.Images) > 0)
	cfg, prov, err := loadChatRuntimeConfig()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err)
		return
	}

	key := projectID + "\x00" + chatID
	run, ok := a.beginRun(key)
	if !ok {
		writeAPIError(w, http.StatusConflict, errors.New("chat is already running"))
		return
	}

	previousImageFiles := cloneImageFiles(sess.ImageFiles)
	previousImageSeq := sess.ImageSeq
	content, err = a.attachIncomingImages(projectID, sess, content, req.Images)
	if err != nil {
		run.cancel()
		a.finishRun(key, run)
		restoreAttachedImages(sess, previousImageFiles, previousImageSeq)
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}

	startContent, startImages := displayContentAndImages(projectID, sess.ID, content, sess.ImageFiles)
	startUser := apiMessage{
		Content: startContent,
		ID:      fmt.Sprintf("stream-user-%d", time.Now().UnixNano()),
		Images:  startImages,
		Role:    "user",
	}
	run.Emit(apiEvent("chat_start", map[string]any{
		"chat": apiChatFromSession(projectID, sess, run),
		"user": startUser,
	}))

	go func() {
		rt := agentruntime.NewRuntime(nil, cfg, prov, projectID, projectRoot, sess)
		rt.Out = io.Discard
		rt.EventSink = run
		rt.InitMCP(run.ctx)

		// Start title generation only after the assistant request is on the
		// wire. This keeps the two requests concurrent without allowing the
		// title request to consume the assistant's first provider response.
		assistantRequestWritten := make(chan struct{})
		var assistantRequestWrittenOnce sync.Once
		assistantCtx := httptrace.WithClientTrace(run.ctx, &httptrace.ClientTrace{
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				if info.Err != nil {
					return
				}
				assistantRequestWrittenOnce.Do(func() { close(assistantRequestWritten) })
			},
		})
		assistantDone := make(chan struct{})
		titleDone := make(chan struct{})
		if generateTitle {
			go func() {
				defer close(titleDone)
				titleCtx, cancel := context.WithTimeout(a.daemonCtx, chatTitleTimeout)
				defer cancel()
				select {
				case <-assistantRequestWritten:
				case <-assistantDone:
				case <-titleCtx.Done():
				}
				if title := rt.FinalizeChatTitle(titleCtx, titleInput); title != "" {
					run.Emit(apiEvent("chat_title", map[string]any{
						"chatID": chatID,
						"title":  title,
					}))
				}
			}()
		} else {
			close(titleDone)
		}

		_ = rt.RunPromptOnce(assistantCtx, content)
		close(assistantDone)
		<-titleDone
		_ = rt.Close()
		if run.ctx.Err() != nil && a.daemonCtx.Err() == nil {
			run.Emit(apiEvent("chat_interrupted", map[string]any{"chatID": chatID}))
		}
		if latest, readErr := chatstore.ReadSession(projectID, chatID); readErr == nil {
			run.Emit(apiEvent("chat_snapshot", map[string]any{
				"chat": apiChatFromSession(projectID, latest, nil),
			}))
		}
		a.finishRun(key, run)
	}()
	a.streamChatRun(w, r, run, 0)
}

func (a *chatAPI) handleChatEvents(w http.ResponseWriter, r *http.Request, projectID, chatID string) {
	startingAfter, err := parseStartingAfter(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	key := projectID + "\x00" + chatID
	if run := a.getRun(key); run != nil {
		a.streamChatRun(w, r, run, startingAfter)
		return
	}

	sess, readErr := chatstore.ReadSession(projectID, chatID)
	if readErr != nil {
		status := http.StatusInternalServerError
		if errors.Is(readErr, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeAPIError(w, status, readErr)
		return
	}
	writeSSEHeaders(w)
	_ = writeSSE(w, apiEvent("chat_snapshot", map[string]any{
		"chat": apiChatFromSession(projectID, sess, nil),
	}))
}

func (a *chatAPI) streamChatRun(w http.ResponseWriter, r *http.Request, run *chatRun, startingAfter int64) {
	replay, events, done, unsubscribe := run.subscribe(r.Context(), startingAfter)
	defer unsubscribe()
	writeSSEHeaders(w)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for _, event := range replay {
		if err := writeSSE(w, event); err != nil {
			return
		}
	}
	for {
		select {
		case event := <-events:
			if err := writeSSE(w, event); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeSSEComment(w, "keep-alive"); err != nil {
				return
			}
		case <-done:
			for {
				select {
				case event := <-events:
					if err := writeSSE(w, event); err != nil {
						return
					}
				default:
					return
				}
			}
		case <-r.Context().Done():
			return
		}
	}
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

func (a *chatAPI) handleChatImage(w http.ResponseWriter, _ *http.Request, projectID, chatID string, seq int) {
	var filePath string
	if sess, err := chatstore.ReadSession(projectID, chatID); err == nil {
		filePath = sess.ImageFiles[seq]
	} else if sub, subErr := chatstore.FindSubSessionByID(projectID, chatID); subErr == nil {
		filePath = sub.ImageFiles[seq]
	}
	if filePath == "" || !isSafeProjectImagePath(projectID, filePath) {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (a *chatAPI) beginRun(key string) (*chatRun, bool) {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	if _, ok := a.active[key]; ok {
		return nil, false
	}
	run := newChatRun(a.daemonCtx)
	a.active[key] = run
	return run, true
}

func (a *chatAPI) finishRun(key string, run *chatRun) {
	run.finish()
	a.activeMu.Lock()
	if a.active[key] == run {
		delete(a.active, key)
	}
	a.activeMu.Unlock()
}

func (a *chatAPI) runActive(key string) bool {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	_, ok := a.active[key]
	return ok
}

func (a *chatAPI) cancelRun(key string) bool {
	a.activeMu.Lock()
	run, ok := a.active[key]
	a.activeMu.Unlock()
	if !ok {
		return false
	}
	run.cancel()
	return true
}

func (a *chatAPI) getRun(key string) *chatRun {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	return a.active[key]
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
		last := sess.LastMessageAt
		if last.IsZero() {
			last = sess.CreatedAt
		}
		title := strings.TrimSpace(sess.Title)
		if title == "" {
			title = "Untitled chat"
		}
		chats = append(chats, apiChatSummary{ID: sess.ID, LastMessageAt: last, Title: title})
	}
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

func projectLastActivity(p apiProject) time.Time {
	var latest time.Time
	for _, chat := range p.Chats {
		if chat.LastMessageAt.After(latest) {
			latest = chat.LastMessageAt
		}
	}
	return latest
}

func apiChatFromSession(projectID string, sess *chatstore.Session, run *chatRun) apiChat {
	if sess == nil {
		return apiChat{ProjectID: projectID, Messages: []apiMessage{}}
	}
	status := ""
	runStartedAt := ""
	if run != nil {
		status = "running"
		runStartedAt = run.startedAt.UTC().Format(time.RFC3339Nano)
	}
	return apiChat{
		CreatedAt:    sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:           sess.ID,
		Messages:     apiMessagesFromSession(projectID, sess.ID, sess.Messages, sess.ImageFiles, sess.UncompactedRaw, run != nil),
		Mode:         "agent",
		ProjectID:    projectID,
		RunStartedAt: runStartedAt,
		Status:       status,
		Title:        sessionTitle(sess),
	}
}

func apiChatFromSubSession(projectID string, sess *chatstore.SubSession) apiChat {
	if sess == nil {
		return apiChat{ProjectID: projectID, Messages: []apiMessage{}}
	}
	return apiChat{
		CreatedAt: sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        sess.ID,
		Messages:  apiMessagesFromSession(projectID, sess.ID, sess.Messages, sess.ImageFiles, nil, chatstore.SubSessionRunning(sess.Status)),
		Mode:      "agent",
		ProjectID: projectID,
		Status:    strings.TrimSpace(sess.Status),
		Title:     strings.TrimSpace(sess.Title),
	}
}

func needsChatTitleGeneration(sess *chatstore.Session) bool {
	if sess == nil || len(sess.Messages) != 0 {
		return false
	}
	title := strings.ToLower(strings.TrimSpace(sess.Title))
	return title == "" || title == "new chat" || title == "untitled chat"
}

func sessionTitle(sess *chatstore.Session) string {
	title := strings.TrimSpace(sess.Title)
	if title == "" {
		return "Untitled chat"
	}
	return title
}

func apiMessagesFromSession(projectID, ownerID string, messages []chatstore.Message, imageFiles map[int]string, dumps []chatstore.UncompactedDump, runActive bool) []apiMessage {
	out := make([]apiMessage, 0, len(messages)+len(dumps)*8)
	for dumpIndex := range dumps {
		archive := archiveMessagesForDisplay(dumps, dumpIndex)
		visibleCount := countDisplayableMessages(archive)
		retainedCount := chatMinInt(8, visibleCount)
		visibleIndex := 0
		for sourceIndex, archivedMessage := range archive {
			if isHiddenSessionMessage(archivedMessage) || isCompactionSummaryMessage(archivedMessage) {
				continue
			}
			if visibleIndex >= visibleCount-retainedCount {
				visibleIndex++
				continue
			}
			if value, ok := apiMessageFromSession(projectID, ownerID, archivedMessage, archive, sourceIndex, imageFiles, runActive, fmt.Sprintf("m-archive-%d-%d", dumpIndex, sourceIndex)); ok {
				out = append(out, value)
			}
			visibleIndex++
		}
		retained := retainedMessagesForDisplay(projectID, ownerID, archive, imageFiles, visibleCount-retainedCount)
		out = append(out, apiMessage{
			Content:          "",
			ID:               fmt.Sprintf("compaction-%d", dumpIndex),
			Kind:             "compaction",
			RetainedMessages: retained,
			Role:             "assistant",
			Summary:          compactionSummary(messages, dumps, dumpIndex),
		})
	}
	for index, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		if value, ok := apiMessageFromSession(projectID, ownerID, message, messages, index, imageFiles, runActive, fmt.Sprintf("m-%d", index)); ok {
			out = append(out, value)
		}
	}
	return out
}

func archiveMessagesForDisplay(dumps []chatstore.UncompactedDump, dumpIndex int) []chatstore.Message {
	if dumpIndex < 0 || dumpIndex >= len(dumps) {
		return nil
	}
	archive := dumps[dumpIndex].Messages
	if dumpIndex == 0 {
		return archive
	}
	for index, message := range archive {
		if isCompactionSummaryMessage(message) {
			return archive[index+1:]
		}
	}
	return archive
}

func countDisplayableMessages(messages []chatstore.Message) int {
	count := 0
	for _, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		count++
	}
	return count
}

func retainedMessagesForDisplay(projectID, ownerID string, messages []chatstore.Message, imageFiles map[int]string, skip int) []apiRetainedMessage {
	retained := make([]apiRetainedMessage, 0, chatMinInt(8, len(messages)))
	visibleIndex := 0
	for _, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		if visibleIndex < skip {
			visibleIndex++
			continue
		}
		content, imgs := displayContentAndImages(projectID, ownerID, message.Content, imageFiles)
		retained = append(retained, apiRetainedMessage{Content: content, Images: imgs, Role: normalizedMessageRole(message.Role)})
		visibleIndex++
	}
	return retained
}

func chatMinInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func apiMessageFromSession(projectID, ownerID string, message chatstore.Message, source []chatstore.Message, index int, imageFiles map[int]string, runActive bool, id string) (apiMessage, bool) {
	if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
		return apiMessage{}, false
	}
	role := normalizedMessageRole(message.Role)
	displaySource := message.Content
	var reasoning string
	if role == "assistant" {
		reasoning, displaySource = chatstore.AssistantDisplayParts(message)
	}
	content, imgs := displayContentAndImages(projectID, ownerID, displaySource, imageFiles)
	value := apiMessage{Content: content, ID: id, Images: imgs, Role: role}
	if !message.CreatedAt.IsZero() {
		value.CreatedAt = message.CreatedAt.Local().Format(time.RFC3339Nano)
	}
	if role == "assistant" {
		value.Reasoning = reasoning
		value.ToolCalls = apiToolCallsFromMessage(projectID, source, index, runActive)
		value.Stats = apiStatsFromMessage(message)
		value.ThoughtFor = message.TurnTTFTSecs
		if value.ThoughtFor == 0 {
			value.ThoughtFor = message.TTFTSecs
		}
		value.WorkedFor = message.TurnWallDisplay
		if value.WorkedFor == 0 {
			value.WorkedFor = message.TurnWallSecs
		}
	}
	if chatstore.MessageCheckpointTagVisible(message) {
		seq := message.CheckpointSeq
		value.CheckpointSeq = &seq
		value.CheckpointBranch = message.CheckpointBranchKey
	}
	return value, true
}

func isHiddenSessionMessage(message chatstore.Message) bool {
	if message.Role == "tool" {
		return true
	}
	return message.Role == "user" && strings.HasPrefix(strings.TrimSpace(message.Content), "tool_result(")
}

func normalizedMessageRole(role string) string {
	if role == "user" {
		return "user"
	}
	return "assistant"
}

func compactionSummary(messages []chatstore.Message, dumps []chatstore.UncompactedDump, dumpIndex int) string {
	var source []chatstore.Message
	if dumpIndex+1 < len(dumps) {
		source = dumps[dumpIndex+1].Messages
	} else {
		source = messages
	}
	for _, message := range source {
		if isCompactionSummaryMessage(message) {
			return compactSummaryText(message.Content)
		}
	}
	return fmt.Sprintf("Context compacted after archived turn %d.", dumpIndex+1)
}

func isCompactionSummaryMessage(message chatstore.Message) bool {
	return message.Role == "assistant" && strings.Contains(message.Content, "[Conversation summary]")
}

func compactSummaryText(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if strings.TrimSpace(line) == "[Conversation summary]" {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(body)
	}
	for index := start; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "[Retained messages]" {
			end = index
			break
		}
	}
	for start < end && isCompactSummarySeparator(lines[start]) {
		start++
	}
	for end > start && isCompactSummarySeparator(lines[end-1]) {
		end--
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func isCompactSummarySeparator(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	first := line[0]
	for index := 1; index < len(line); index++ {
		if line[index] != first {
			return false
		}
	}
	return first == '-' || first == '='
}

func displayContentAndImages(projectID, ownerID, content string, imageFiles map[int]string) (string, []apiImage) {
	imageValues := make([]apiImage, 0)
	display := images.ExpandForDisplay(content)
	seen := make(map[int]bool)
	display = imageTagPattern.ReplaceAllStringFunc(display, func(tag string) string {
		match := imageTagPattern.FindStringSubmatch(tag)
		if len(match) != 2 {
			return tag
		}
		seq, err := strconv.Atoi(match[1])
		if err != nil || imageFiles == nil {
			return ""
		}
		filePath := strings.TrimSpace(imageFiles[seq])
		if filePath == "" || !isSafeProjectImagePath(projectID, filePath) {
			return ""
		}
		if !seen[seq] {
			seen[seq] = true
			name := filepath.Base(filePath)
			if name == "." || name == string(filepath.Separator) {
				name = fmt.Sprintf("image-%d", seq)
			}
			imageValues = append(imageValues, apiImage{
				Name: name,
				URL:  fmt.Sprintf("%s/%s/chats/%s/images/%d", chatAPIPath, url.PathEscape(projectID), url.PathEscape(ownerID), seq),
			})
		}
		return ""
	})
	return strings.TrimSpace(display), imageValues
}

func chatTitleInput(content string, hasImages bool) string {
	content = imageTagPattern.ReplaceAllString(content, "image")
	content = strings.TrimSpace(content)
	if content == "" && hasImages {
		return "The user shared an image and needs help with it."
	}
	return content
}

func apiStatsFromMessage(message chatstore.Message) *apiStats {
	if !message.TurnDisplaySaved && message.PromptTokens == 0 && message.UserPromptTokens == 0 && message.ReasoningTokens == 0 && message.ResponseTokens == 0 && message.TurnTotalTokens == 0 && message.TurnWallSecs == 0 {
		return nil
	}
	contextTokens := message.TurnContextTokens
	if contextTokens == 0 {
		contextTokens = message.PromptTokens
	}
	userTokens := message.TurnUserTokens
	if userTokens == 0 {
		userTokens = message.UserPromptTokens
	}
	reasoningTokens := message.TurnReasonTokens
	if reasoningTokens == 0 {
		reasoningTokens = message.ReasoningTokens
	}
	responseTokens := message.TurnRespTokens
	if responseTokens == 0 {
		responseTokens = message.ResponseTokens
	}
	totalTokens := message.TurnTotalDisplay
	if totalTokens == 0 {
		totalTokens = message.TurnTotalTokens
	}
	outputTPS := message.TurnOutputTPS
	if outputTPS == 0 {
		outputTPS = message.OutputTPS
	}
	ttft := message.TurnTTFTSecs
	if ttft == 0 {
		ttft = message.TTFTSecs
	}
	promptTPS := message.TurnPromptTPS
	if promptTPS == 0 {
		promptTPS = message.PromptTPS
	}
	return &apiStats{
		ContextTokens: contextTokens, OutputTokensPerSecond: outputTPS, PromptTokensPerSecond: promptTPS,
		ReasoningTokens: reasoningTokens, ResponseTokens: responseTokens, TotalTokens: totalTokens,
		TTFTSeconds: ttft, UserTokens: userTokens,
	}
}

func apiToolCallsFromMessage(projectID string, messages []chatstore.Message, messageIndex int, runActive bool) []apiToolCall {
	message := messages[messageIndex]
	if len(message.ToolCalls) == 0 {
		return nil
	}
	results := make(map[string]chatstore.Message)
	for _, candidate := range messages[messageIndex+1:] {
		if candidate.Role == "user" {
			break
		}
		if candidate.Role == "tool" && candidate.ToolCallID != "" {
			results[candidate.ToolCallID] = candidate
		}
	}
	out := make([]apiToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		args := decodeArguments(call.Arguments)
		tool := apiToolCall{
			ID:         call.ID,
			Name:       call.Name,
			Input:      toolInput(call.Name, args, call.Arguments),
			Intent:     stringField(args, "intent"),
			Mode:       stringField(args, "mode"),
			NewString:  stringField(args, "newString"),
			OldString:  stringField(args, "oldString"),
			RenameTo:   stringField(args, "renameTo"),
			Delete:     boolField(args, "delete"),
			Full:       boolField(args, "full"),
			Parameters: toolParameters(call.Name, args),
		}
		if call.CpSeqSet || call.CheckpointSeq > 0 {
			checkpointSeq := call.CheckpointSeq
			tool.CheckpointSeq = &checkpointSeq
			tool.CheckpointBranch = call.CheckpointBranchKey
		}
		if call.Name == "subagent" && subagentIsSynchronous(args) {
			tool.Sync = true
		}
		if resultMessage, ok := results[call.ID]; ok {
			tool.Result = apiToolResult(resultMessage.Content)
			if resultStatus, ok := tool.Result["status"].(string); ok && resultStatus == "error" {
				tool.Status = "error"
			} else {
				tool.Status = "success"
			}
		} else if err := tooling.ValidateToolIntent(json.RawMessage(call.Arguments)); err != nil {
			// Older interrupted/rejected turns may have persisted the assistant
			// tool call before dispatch validation. Do not resurrect those calls
			// as active work when the real chat is reopened.
			tool.Result = map[string]any{
				"error":  err.Error(),
				"status": "error",
			}
			tool.Status = "error"
		} else if !runActive {
			// A completed chat can retain an assistant tool call if the process
			// stopped after persisting the assistant turn but before persisting
			// its result. It is not live work anymore, so do not resurrect it as
			// running when the real chat is reopened.
			tool.Result = map[string]any{
				"error":  "chat run ended before the tool result was persisted",
				"status": "error",
			}
			tool.Status = "error"
		} else {
			tool.Status = "running"
		}
		if tool.Name == "subagent" && tool.Result != nil {
			if subchatID := stringField(tool.Result, "subchatId"); subchatID != "" {
				if subSession, subErr := chatstore.FindSubSessionByID(projectID, subchatID); subErr == nil && subSession != nil && strings.TrimSpace(subSession.Status) != "" {
					tool.Result["subagentStatus"] = strings.TrimSpace(subSession.Status)
				}
			}
			if subStatus, ok := tool.Result["subagentStatus"].(string); ok && chatstore.SubSessionRunning(subStatus) {
				tool.Status = "running"
			}
		}
		out = append(out, tool)
	}
	return out
}

func decodeArguments(raw string) map[string]any {
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil || args == nil {
		return map[string]any{}
	}
	return args
}

func toolInput(name string, args map[string]any, raw string) string {
	keys := []string{"source", "command", "path", "task", "query", "name", "pattern", "url", "id"}
	for _, key := range keys {
		if value := stringField(args, key); value != "" {
			return value
		}
	}
	if name == "listSubAgents" || name == "todoList" {
		return ""
	}
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return ""
	}
	return strings.TrimSpace(raw)
}

func toolParameters(name string, args map[string]any) []apiToolParameter {
	var keys []string
	switch name {
	case "find", "createPlan", "addTodo", "fetchWeb", "webSearch":
		for key := range args {
			if key != "intent" && key != "mode" {
				keys = append(keys, key)
			}
		}
	default:
		return nil
	}
	sort.Strings(keys)
	params := make([]apiToolParameter, 0, len(keys))
	for _, key := range keys {
		params = append(params, apiToolParameter{Label: key, Value: fmt.Sprint(args[key])})
	}
	return params
}

func apiToolResult(raw string) map[string]any {
	var decoded any
	if json.Unmarshal([]byte(raw), &decoded) != nil {
		return map[string]any{"output": raw, "status": "success"}
	}
	result, ok := decoded.(map[string]any)
	if !ok {
		b, _ := json.Marshal(decoded)
		return map[string]any{"output": string(b), "status": "success"}
	}
	out := make(map[string]any, len(result)+2)
	for key, value := range result {
		out[key] = value
	}
	if calls, ok := result["tool_calls"].([]any); ok {
		out["sdkCalls"] = len(calls)
	}
	if compileError := stringField(result, "compile_error"); compileError != "" && stringField(result, "error") == "" {
		out["error"] = compileError
	}
	status := "success"
	if errText := stringField(result, "error"); errText != "" {
		status = "error"
	}
	if okValue, exists := result["ok"].(bool); exists && !okValue {
		status = "error"
	}
	if statusValue, exists := result["status"].(string); exists && (statusValue == "error" || statusValue == "failed") {
		status = "error"
	}
	if subStatus := stringField(result, "status"); subStatus != "" && subStatus != "success" && subStatus != "error" {
		out["subagentStatus"] = subStatus
	}
	out["status"] = status
	if _, exists := out["output"]; !exists {
		if value, exists := out["summary"]; exists {
			out["output"] = fmt.Sprint(value)
		}
	}
	return out
}

func stringField(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if ok {
		return text
	}
	return ""
}

func boolField(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func subagentIsSynchronous(args map[string]any) bool {
	value, ok := args["run_in_background"]
	if !ok {
		return true
	}
	switch value := value.(type) {
	case bool:
		return !value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return false
		case "0", "false", "no", "off":
			return true
		}
	}
	return true
}

func loadChatRuntimeConfig() (*config.Root, *config.Provider, error) {
	cfg, err := config.LoadOptional()
	if err != nil {
		return nil, nil, err
	}
	if config.NeedsOnboard(cfg) {
		return nil, nil, errors.New("config not set up; use /onboard")
	}
	prov, err := config.ResolveProvider(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, prov, nil
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

func isSafeProjectImagePath(projectID, filePath string) bool {
	root, err := paths.ChatImagesDir(projectID)
	if err != nil {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(base, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (a *chatAPI) attachIncomingImages(projectID string, sess *chatstore.Session, content string, incoming []incomingImage) (string, error) {
	if len(incoming) == 0 {
		return content, nil
	}
	tags := imageTagPattern.FindAllStringSubmatch(content, -1)
	created := make([]string, 0, len(incoming))
	cleanup := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	nextSeq := sess.ImageSeq
	nextContent := content
	addedFiles := make(map[int]string, len(incoming))
	for index, incomingImage := range incoming {
		dataURL := strings.TrimSpace(incomingImage.Data)
		if dataURL == "" {
			dataURL = strings.TrimSpace(incomingImage.URL)
		}
		data, err := decodeImageDataURL(dataURL)
		if err != nil {
			cleanup()
			return content, err
		}
		if _, ok := images.MIMEForBinary(data); !ok {
			cleanup()
			return content, errors.New("unsupported image attachment")
		}
		seq := nextSeq
		nextSeq++
		filePath, err := writeIncomingImage(projectID, sess.ID, seq, data)
		if err != nil {
			cleanup()
			return content, err
		}
		created = append(created, filePath)
		addedFiles[seq] = filePath
		if index < len(tags) {
			nextContent = strings.ReplaceAll(nextContent, tags[index][0], images.VisibleTag(seq))
		} else {
			nextContent = strings.TrimSpace(nextContent + " " + images.VisibleTag(seq))
		}
	}
	if sess.ImageFiles == nil {
		sess.ImageFiles = make(map[int]string, len(addedFiles))
	}
	for seq, filePath := range addedFiles {
		sess.ImageFiles[seq] = filePath
	}
	sess.ImageSeq = nextSeq
	return nextContent, nil
}

func writeIncomingImage(projectID, chatID string, seq int, data []byte) (string, error) {
	stamp := time.Now().UTC()
	for attempt := 0; attempt < 1000; attempt++ {
		filePath, err := paths.ImagePath(projectID, chatID, seq, stamp)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			return "", err
		}
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			stamp = stamp.Add(time.Millisecond)
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(filePath)
			return "", writeErr
		}
		if closeErr != nil {
			_ = os.Remove(filePath)
			return "", closeErr
		}
		return filePath, nil
	}
	return "", errors.New("unable to allocate a unique image path")
}

func cloneImageFiles(files map[int]string) map[int]string {
	if len(files) == 0 {
		return nil
	}
	clone := make(map[int]string, len(files))
	for seq, filePath := range files {
		clone[seq] = filePath
	}
	return clone
}

func restoreAttachedImages(sess *chatstore.Session, previous map[int]string, previousSeq int) {
	if sess == nil {
		return
	}
	for seq, filePath := range sess.ImageFiles {
		if previous[seq] != filePath {
			_ = os.Remove(filePath)
		}
	}
	sess.ImageFiles = cloneImageFiles(previous)
	sess.ImageSeq = previousSeq
}

func decodeImageDataURL(value string) ([]byte, error) {
	if !strings.HasPrefix(value, "data:") {
		return nil, errors.New("image attachment must be a data URL")
	}
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 || !strings.Contains(parts[0], ";base64") {
		return nil, errors.New("invalid image data URL")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid image data")
	}
	if len(data) == 0 {
		return nil, errors.New("empty image attachment")
	}
	return data, nil
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

type httpEventSink struct {
	ctx     context.Context
	events  chan cievents.Event
	once    sync.Once
	stateMu sync.RWMutex
	closed  bool
}

func newHTTPEventSink(ctx context.Context) *httpEventSink {
	return &httpEventSink{ctx: ctx, events: make(chan cievents.Event, 256)}
}

func (s *httpEventSink) Emit(ev cievents.Event) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.closed {
		return
	}
	select {
	case s.events <- ev:
	case <-s.ctx.Done():
	}
}

func (s *httpEventSink) StreamMode() bool { return true }

func (s *httpEventSink) Events() []cievents.Event { return nil }

func (s *httpEventSink) FlushReport(cievents.ReportMeta, int, string, string, any) error { return nil }

func (s *httpEventSink) Close() {
	s.once.Do(func() {
		s.stateMu.Lock()
		s.closed = true
		close(s.events)
		s.stateMu.Unlock()
	})
}

func (s *httpEventSink) EventsChannel() <-chan cievents.Event { return s.events }

func apiEvent(typ string, fields map[string]any) cievents.Event {
	event := cievents.Event{"v": cievents.SchemaVersion, "type": typ, "ts": time.Now().UTC().Format(time.RFC3339Nano)}
	for key, value := range fields {
		event[key] = value
	}
	return event
}

func writeSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

func writeSSE(w http.ResponseWriter, event cievents.Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeSSEComment(w http.ResponseWriter, comment string) error {
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
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
	writeJSON(w, status, map[string]string{"error": message})
}
