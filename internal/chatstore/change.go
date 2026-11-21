package chatstore

import (
	"database/sql"
	"errors"
)

type mainMessageChange struct {
	previousLength int
	first          int
}

var errChatPositionMismatch = errors.New("stored chat position changed")

func WriteMainMessageChange(project string, session *Session, previousLength, first int) error {
	session.Capabilities.Backfill(session.Messages)
	err := storeChat(project, session, false, mainMessageChange{previousLength, first})
	if errors.Is(err, errChatPositionMismatch) {
		return WriteSession(project, session)
	}
	return err
}

func syncMainMessageChange(tx *sql.Tx, home, key string, streams []messageStream, items [][]Message, change mainMessageChange) error {
	var length int
	if err := tx.QueryRow(`
SELECT COALESCE(MAX(position) + 1, 0) FROM chat_membership WHERE session_id = ? AND stream = 'main'`, key).Scan(&length); err != nil {
		return err
	}
	messages := items[0]
	if length != change.previousLength || change.first < 0 || change.first > length || len(messages) < length {
		return errChatPositionMismatch
	}
	end := len(messages)
	var replaced string
	if change.first < length {
		if len(messages) != length {
			return errChatPositionMismatch
		}
		end = change.first + 1
		err := tx.QueryRow(`
SELECT message_id FROM chat_membership WHERE session_id = ? AND stream = 'main' AND position = ?`, key, change.first).Scan(&replaced)
		if errors.Is(err, sql.ErrNoRows) {
			return errChatPositionMismatch
		}
		if err != nil {
			return err
		}
	}
	for position := change.first; position < end; position++ {
		record, err := encodeChatMessage(home, key, messages[position])
		if err != nil {
			return err
		}
		if record.id == replaced {
			return nil
		}
		if err := writeChatPosition(tx, key, messagePosition{streams[0].name, position}, record); err != nil {
			return err
		}
	}
	if replaced != "" {
		_, err := tx.Exec(`
DELETE FROM chat_message WHERE session_id = ? AND sha256 = ? AND NOT EXISTS (
    SELECT 1 FROM chat_membership WHERE session_id = ? AND message_id = ?
)`, key, replaced, key, replaced)
		return err
	}
	return nil
}
