package chatstore

import (
	"database/sql"
	"encoding/json"
)

type storedMessage struct {
	id, role, content, metadata string
	epoch                       any
}

type messagePosition struct {
	stream string
	index  int
}

func chatPositions(tx *sql.Tx, key string) (map[messagePosition]string, error) {
	rows, err := tx.Query("SELECT stream, position, message_id FROM chat_membership WHERE session_id = ?", key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[messagePosition]string)
	for rows.Next() {
		var position messagePosition
		var id string
		if err := rows.Scan(&position.stream, &position.index, &id); err != nil {
			return nil, err
		}
		result[position] = id
	}
	return result, rows.Err()
}

func syncChatMessages(tx *sql.Tx, home, key string, streams []messageStream, items [][]Message) error {
	existing, err := chatPositions(tx, key)
	if err != nil {
		return err
	}
	mayHaveOrphans := false
	for i, stream := range streams {
		for index, message := range items[i] {
			position := messagePosition{stream.name, index}
			previous := existing[position]
			delete(existing, position)
			record, err := encodeChatMessage(home, key, message)
			if err != nil {
				return err
			}
			if previous == record.id {
				continue
			}
			if err := writeChatPosition(tx, key, position, record); err != nil {
				return err
			}
			mayHaveOrphans = mayHaveOrphans || previous != ""
		}
	}
	for position := range existing {
		if _, err := tx.Exec("DELETE FROM chat_membership WHERE session_id = ? AND stream = ? AND position = ?", key, position.stream, position.index); err != nil {
			return err
		}
	}
	if mayHaveOrphans || len(existing) > 0 {
		return removeOrphanChatMessages(tx, key)
	}
	return nil
}

func removeOrphanChatMessages(tx *sql.Tx, key string) error {
	_, err := tx.Exec(`
DELETE FROM chat_message WHERE session_id = ? AND sha256 NOT IN
(SELECT message_id FROM chat_membership WHERE session_id = ?)`, key, key)
	return err
}

func writeChatPosition(tx *sql.Tx, key string, position messagePosition, record storedMessage) error {
	if _, err := tx.Exec(`
INSERT INTO chat_message(sha256, session_id, epoch_ns, role, content, metadata) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(sha256) DO NOTHING`, record.id, key, record.epoch, record.role, record.content, record.metadata); err != nil {
		return err
	}
	_, err := tx.Exec(`
INSERT INTO chat_membership(session_id, stream, position, message_id) VALUES (?, ?, ?, ?)
ON CONFLICT(session_id, stream, position) DO UPDATE SET message_id = excluded.message_id`,
		key, position.stream, position.index, record.id)
	return err
}

func encodeChatMessage(home, session string, message Message) (storedMessage, error) {
	images, err := storeAttachments(home, message.ToolImages)
	if err != nil {
		return storedMessage{}, err
	}
	media, err := storeAttachments(home, message.MediaPreviews)
	if err != nil {
		return storedMessage{}, err
	}
	role, content := message.Role, message.Content
	var epoch any
	if !message.CreatedAt.IsZero() {
		epoch = message.CreatedAt.UnixNano()
	}
	message.Role, message.Content = "", ""
	message.ToolImages, message.MediaPreviews = nil, nil
	body, err := json.Marshal(messageMetadata{Message: message, Images: images, Media: media})
	if err != nil {
		return storedMessage{}, err
	}
	id, err := chatDigest(session, epoch, role, content, string(body))
	if err != nil {
		return storedMessage{}, err
	}
	return storedMessage{id: id, epoch: epoch, role: role, content: content, metadata: string(body)}, nil
}
