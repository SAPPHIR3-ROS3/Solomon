package agent

import (
	"context"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/title"
)

func (r *Runtime) scheduleDeferredChatTitleFinalize(ctx context.Context) {
	r.deferredTitleScheduleMu.Lock()
	if !chatstore.IsPlaceholderChatID(r.Session.ID) || r.deferredTitleWorkerRunning {
		r.deferredTitleScheduleMu.Unlock()
		return
	}
	r.deferredTitleWorkerRunning = true
	r.deferredTitleScheduleMu.Unlock()
	go r.runDeferredChatTitleFinalize(ctx)
}

func (r *Runtime) runDeferredChatTitleFinalize(ctx context.Context) {
	defer func() {
		r.deferredTitleScheduleMu.Lock()
		r.deferredTitleWorkerRunning = false
		r.deferredTitleScheduleMu.Unlock()
	}()

	r.chatPersistMu.Lock()
	var firstUser string
	for _, m := range r.Session.Messages {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" && !strings.HasPrefix(m.Content, "tool_result(") {
			firstUser = m.Content
			break
		}
	}
	oldID := r.Session.ID
	r.chatPersistMu.Unlock()

	if firstUser == "" {
		return
	}

	t, err := title.FromPrompt(ctx, r.Client, r.Cfg, r.Model, firstUser)
	if err != nil || strings.TrimSpace(t) == "" {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "deferred chat title FromPrompt failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
		t = title.FallbackFromWords(firstUser)
	}
	t = title.NormalizeSlug(t)

	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()

	if !chatstore.IsPlaceholderChatID(r.Session.ID) || r.Session.ID != oldID {
		return
	}

	r.Session.Title = t
	newChatID := chatstore.ChatIDHex(t, r.Session.CreatedAt)
	if len(r.Session.ImageFiles) > 0 {
		if err := chatstore.MigrateImagePathsAfterChatRename(r.ProjHex, r.Session, oldID, newChatID); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "migrate pasted image paths after rename failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}
	r.Session.ID = newChatID
	if err := chatstore.RenameSessionFile(r.ProjHex, oldID, r.Session.ID); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "rename session file failed", logging.LogOptions{Params: map[string]any{"old_id": oldID, "new_id": r.Session.ID, "err": err.Error()}})
		if err2 := r.persistSessionUnsafe(); err2 != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "persist session after rename failure failed", logging.LogOptions{Params: map[string]any{"err": err2.Error()}})
		}
		_ = chatstore.RemoveSessionPath(r.ProjHex, oldID)
		return
	}
	if err := r.persistSessionUnsafe(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session after title finalize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
}

func (r *Runtime) refineEphemeralTitle(ctx context.Context, firstUserLine string) {
	firstUserLine = strings.TrimSpace(firstUserLine)
	t, err := title.FromPrompt(ctx, r.Client, r.Cfg, r.Model, firstUserLine)
	if err != nil || strings.TrimSpace(t) == "" {
		return
	}
	t = title.NormalizeSlug(strings.TrimSpace(t))

	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()

	if len(r.Session.Messages) == 0 {
		return
	}
	u0 := r.Session.Messages[0]
	if u0.Role != "user" || strings.TrimSpace(u0.Content) != firstUserLine {
		return
	}
	r.Session.Title = t
}
