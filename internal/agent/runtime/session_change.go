package agentruntime

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

func (r *Runtime) saveMainMessageChange(fn func(*chatstore.Session) int) error {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	previousLength := len(r.Session.Messages)
	first := fn(r.Session)
	if !r.shouldWriteSessionFile() {
		r.mainMessagesSynced = false
		return nil
	}
	if !r.mainMessagesSynced || first < 0 {
		return r.writeSessionLocked()
	}
	if err := chatstore.WriteMainMessageChange(r.ProjHex, r.Session, previousLength, first); err != nil {
		r.mainMessagesSynced = false
		return err
	}
	return r.rememberPersistedChat()
}
