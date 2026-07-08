package test

import (
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestSessionFileLockConflict(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projHex := "0000000000000000000000000000000000000000000000000000000000000000"
	chatID := "chat-lock-test"
	sess := &chatstore.Session{ID: chatID}
	if err := chatstore.WriteSession(projHex, sess); err != nil {
		t.Fatal(err)
	}
	lock1, err := chatstore.TryAcquireSessionFileLock(projHex, chatID)
	if err != nil {
		t.Fatal(err)
	}
	defer lock1.Release()
	lock2, err := chatstore.TryAcquireSessionFileLock(projHex, chatID)
	if err == nil {
		if lock2 != nil {
			lock2.Release()
		}
		t.Fatal("expected lock conflict")
	}
}

func TestSessionFileLockPlaceholderSkipped(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projHex := "0000000000000000000000000000000000000000000000000000000000000000"
	lock, err := chatstore.TryAcquireSessionFileLock(projHex, chatstore.NewPlaceholderChatID(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if lock != nil {
		lock.Release()
		t.Fatal("placeholder chat should not acquire lock")
	}
}
