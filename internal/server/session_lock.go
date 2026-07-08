package server

import (
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type sessionLock struct {
	inner *chatstore.SessionFileLock
}

func acquireSessionLock(projectHex, chatID string) (*sessionLock, error) {
	fl, err := chatstore.TryAcquireSessionFileLock(projectHex, chatID)
	if err != nil {
		return nil, err
	}
	if fl == nil {
		return nil, nil
	}
	return &sessionLock{inner: fl}, nil
}

func (l *sessionLock) Release() {
	if l == nil || l.inner == nil {
		return
	}
	l.inner.Release()
}

func conversationIDForSession(sess *chatstore.Session) string {
	if sess == nil {
		return ""
	}
	return sess.ID
}

func newConversationSession() *chatstore.Session {
	now := time.Now()
	return &chatstore.Session{
		ID:             chatstore.NewPlaceholderChatID(now),
		CreatedAt:      now,
		LastMessageAt:  now,
		CheckpointLast: -1,
		CheckpointCP0:  true,
	}
}
