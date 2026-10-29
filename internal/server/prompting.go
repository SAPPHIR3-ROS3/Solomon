package server

import (
	"context"
	"errors"
	"fmt"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"time"
)

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
	if slash.IsCommand(content) {
		if len(req.Images) != 0 || len(req.Clips) != 0 {
			writeAPIError(w, http.StatusBadRequest, errors.New("send attachments as a message, separately from slash commands"))
			return
		}
		a.handleSendSlashCommand(w, r, projectID, projectRoot, chatID, sess, content, clientSurface(r, req.Surface))
		return
	}
	titleInput := chatTitleInput(req.Content, len(req.Images) > 0)
	surface := clientSurface(r, req.Surface)
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
	sess.TerminalClips = atmention.MergeTerminalClips(sess.TerminalClips, incomingClips(req.Clips))

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
		rt.Surface = surface
		a.attachBrowser(rt, projectID, chatID)
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
