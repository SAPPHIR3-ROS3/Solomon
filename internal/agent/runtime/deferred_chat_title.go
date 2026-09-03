package agentruntime

import (
	"context"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/title"
)

func (r *Runtime) scheduleDeferredChatTitleFinalize(ctx context.Context) {
	r.deferredTitleScheduleMu.Lock()
	if r.deferredTitleWorkerRunning {
		r.deferredTitleScheduleMu.Unlock()
		return
	}
	r.chatPersistMu.Lock()
	start := chatstore.IsPlaceholderChatID(r.Session.ID)
	if start {
		r.deferredTitleWorkerRunning = true
	}
	r.chatPersistMu.Unlock()
	r.deferredTitleScheduleMu.Unlock()
	if !start {
		return
	}
	go r.runDeferredChatTitleFinalize(context.WithoutCancel(ctx))
}

// FinalizeChatTitle generates a title for an API-created chat while keeping
// its existing ID stable. The API needs a stable ID so the browser can keep
// its stream attached while the title is refined.
func (r *Runtime) FinalizeChatTitle(ctx context.Context, firstUserLine string) string {
	firstUserLine = strings.TrimSpace(firstUserLine)
	if r == nil || r.Session == nil || firstUserLine == "" {
		return ""
	}
	providerReady := r.waitProviderReady(ctx) == nil

	t := ""
	var err error
	if providerReady && r.Backend != nil {
		t, err = title.FromPrompt(ctx, r.Backend, r.Client, r.Cfg, r.Model, firstUserLine)
	}
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "chat title FromPrompt failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
	if strings.TrimSpace(t) == "" {
		t = title.FallbackFromWords(firstUserLine)
	}
	t = title.NormalizeSlug(t)

	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	r.Session.Title = t
	if err := r.writeSessionLocked(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist chat title failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
	return t
}

func (r *Runtime) runDeferredChatTitleFinalize(ctx context.Context) {
	titleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	defer func() {
		r.deferredTitleScheduleMu.Lock()
		r.deferredTitleWorkerRunning = false
		r.deferredTitleScheduleMu.Unlock()
	}()

	select {
	case <-time.After(5 * time.Second):
	case <-titleCtx.Done():
		return
	}

	var firstUser string
	var oldID string
	r.chatPersistMu.Lock()
	for _, m := range r.Session.Messages {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" && !strings.HasPrefix(m.Content, "tool_result(") {
			firstUser = m.Content
			break
		}
	}
	oldID = r.Session.ID
	r.chatPersistMu.Unlock()

	if firstUser == "" {
		return
	}

	if err := r.waitProviderReady(titleCtx); err != nil {
		return
	}
	t, err := title.FromPrompt(titleCtx, r.Backend, r.Client, r.Cfg, r.Model, firstUser)
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
	if err := r.waitProviderReady(ctx); err != nil {
		return
	}
	t, err := title.FromPrompt(ctx, r.Backend, r.Client, r.Cfg, r.Model, firstUserLine)
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
