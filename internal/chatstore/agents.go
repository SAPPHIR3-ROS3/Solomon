package chatstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
)

func agentScope(project, origin string) string {
	if origin == SubOriginScheduled {
		return "scheduled"
	}
	return "project/" + project
}

func storeAgent(project string, agent *SubSession, legacy bool) error {
	db, home, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	scope := agentScope(project, agent.Origin)
	var key string
	err = tx.QueryRow("SELECT sha256 FROM agent_session WHERE storage_scope = ? AND agent_id = ?", scope, agent.ID).Scan(&key)
	if err == nil && legacy {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if key == "" {
		key, err = chatDigest(scope, agent.ID, agent.CreatedAt.UnixNano(), agent.Origin)
		if err != nil {
			return err
		}
	}
	metadata := *agent
	if metadata.ProjectHex == "" {
		metadata.ProjectHex = project
	}
	metadata.Messages, metadata.TaskQueue, metadata.PendingSpawns = nil, nil, nil
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO agent_session(storage_scope, agent_id, sha256, project_id, origin, parent_chat_id, parent_subchat_id,
parent_tool_call_id, status, epoch_ns, format_version, metadata)
VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, 1, ?)
ON CONFLICT(storage_scope, agent_id) DO UPDATE SET project_id=excluded.project_id,
origin=excluded.origin, parent_chat_id=excluded.parent_chat_id, parent_subchat_id=excluded.parent_subchat_id,
parent_tool_call_id=excluded.parent_tool_call_id, status=excluded.status, metadata=excluded.metadata`,
		scope, agent.ID, key, metadata.ProjectHex, metadata.Origin, metadata.ParentChatID, metadata.ParentSubchatID,
		metadata.ParentToolCallID, metadata.Status, metadata.CreatedAt.UnixNano(), string(raw))
	if err != nil {
		return err
	}
	for _, table := range []string{"agent_message", "agent_queue"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE session_id = ?", key); err != nil {
			return err
		}
	}
	for i, message := range agent.Messages {
		images, err := storeAttachments(home, message.ToolImages)
		if err != nil {
			return err
		}
		media, err := storeAttachments(home, message.MediaPreviews)
		if err != nil {
			return err
		}
		message.ToolImages, message.MediaPreviews = nil, nil
		data, err := json.Marshal(messageMetadata{Message: message, Images: images, Media: media})
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO agent_message(session_id, position, message) VALUES (?, ?, ?)", key, i, string(data)); err != nil {
			return err
		}
	}
	if err := storeAgentQueue(tx, key, "task", agent.TaskQueue); err != nil {
		return err
	}
	if err := storeAgentQueue(tx, key, "spawn", agent.PendingSpawns); err != nil {
		return err
	}
	return tx.Commit()
}

func storeAgentQueue[T any](tx *sql.Tx, key, kind string, operations []T) error {
	for i, operation := range operations {
		data, err := json.Marshal(operation)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO agent_queue(session_id, kind, position, operation) VALUES (?, ?, ?, ?)", key, kind, i, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func readAgent(project, origin, id string) (*SubSession, error) {
	db, home, err := openChatDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var key, raw string
	err = tx.QueryRow("SELECT sha256, metadata FROM agent_session WHERE storage_scope = ? AND agent_id = ?", agentScope(project, origin), id).Scan(&key, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	var agent SubSession
	if err := json.Unmarshal([]byte(raw), &agent); err != nil {
		return nil, err
	}
	messages, err := readAgentRows[messageMetadata](tx, "SELECT message FROM agent_message WHERE session_id = ? ORDER BY position", key)
	if err != nil {
		return nil, err
	}
	for _, item := range messages {
		message := item.Message
		message.ToolImages, err = restoreAttachments(home, item.Images)
		if err != nil {
			return nil, err
		}
		message.MediaPreviews, err = restoreAttachments(home, item.Media)
		if err != nil {
			return nil, err
		}
		agent.Messages = append(agent.Messages, message)
	}
	const queue = "SELECT operation FROM agent_queue WHERE session_id = ? AND kind = ? ORDER BY position"
	agent.TaskQueue, err = readAgentRows[QueuedTask](tx, queue, key, "task")
	if err != nil {
		return nil, err
	}
	agent.PendingSpawns, err = readAgentRows[PendingSubagentSpawn](tx, queue, key, "spawn")
	if err != nil {
		return nil, err
	}
	return &agent, tx.Commit()
}

func readAgentRows[T any](tx *sql.Tx, query string, args ...any) ([]T, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []T
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func agentIDs(project, origin string) ([]string, error) {
	db, _, err := openChatDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT agent_id FROM agent_session WHERE storage_scope = ? ORDER BY agent_id", agentScope(project, origin))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}
