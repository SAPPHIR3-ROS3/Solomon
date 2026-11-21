package chatstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const chatFormatVersion = 1

var interactiveChatWrites atomic.Int64

func openChatDatabase() (*database.Lease, string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, "", err
	}
	db, err := database.BorrowData(context.Background(), home)
	return db, home, err
}

func chatDigest(fields ...any) (string, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func storeChat(project string, session *Session, legacy bool, change ...mainMessageChange) error {
	if !legacy {
		interactiveChatWrites.Add(1)
		defer interactiveChatWrites.Add(-1)
	}
	db, home, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	return storeChatWithDB(context.Background(), db.DB, home, project, session, legacy, change...)
}

type chatTransactionStarter interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func storeChatWithDB(ctx context.Context, db chatTransactionStarter, home, project string, session *Session, legacy bool, change ...mainMessageChange) error {
	metadata := copySessionStructure(session)
	streams := streamsFor(&metadata)
	usage := sessionUsageTotals(session)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var key string
	err = tx.QueryRow("SELECT sha256 FROM chat_session WHERE project_id = ? AND chat_id = ?", project, session.ID).Scan(&key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if legacy {
		var known bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_legacy_alias WHERE project_id = ? AND chat_id = ?)", project, session.ID).Scan(&known); err != nil {
			return err
		}
		if known || key != "" {
			return nil
		}
	}
	if key == "" {
		if len(change) != 0 {
			return errChatPositionMismatch
		}
		key, err = chatDigest(project, session.ID, session.CreatedAt.UnixNano())
		if err != nil {
			return err
		}
	}
	items := make([][]Message, len(streams))
	for i, stream := range streams {
		items[i] = *stream.messages
		if items[i] != nil {
			*stream.messages = []Message{}
		}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO chat_session(sha256, project_id, chat_id, epoch_ns, format_version, title, last_activity, last_user_activity, metadata, sidebar_activity_ns)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(sha256) DO UPDATE SET title = excluded.title, last_activity = excluded.last_activity,
last_user_activity = excluded.last_user_activity, metadata = excluded.metadata,
sidebar_activity_ns = excluded.sidebar_activity_ns
WHERE chat_session.title IS NOT excluded.title OR chat_session.last_activity IS NOT excluded.last_activity
OR chat_session.last_user_activity IS NOT excluded.last_user_activity OR chat_session.metadata IS NOT excluded.metadata`,
		key, project, session.ID, session.CreatedAt.UnixNano(), chatFormatVersion, session.Title,
		SessionActivityTime(session).UTC().Format(time.RFC3339Nano),
		summaryLastUserActivity(session).UTC().Format(time.RFC3339Nano), string(raw), sidebarActivity(session).UnixNano())
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO chat_usage(session_id, user_tokens, reasoning_tokens, response_tokens) VALUES (?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET user_tokens=excluded.user_tokens,
reasoning_tokens=excluded.reasoning_tokens, response_tokens=excluded.response_tokens
WHERE chat_usage.user_tokens IS NOT excluded.user_tokens OR chat_usage.reasoning_tokens IS NOT excluded.reasoning_tokens
OR chat_usage.response_tokens IS NOT excluded.response_tokens`,
		key, usage.UserSum, usage.ReasonSum, usage.RespSum); err != nil {
		return err
	}
	if len(change) == 0 {
		if err := syncChatMessages(tx, home, key, streams, items); err != nil {
			return err
		}
	} else if err := syncMainMessageChange(tx, home, key, streams, items, change[0]); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO chat_legacy_alias(project_id, chat_id, session_id) VALUES (?, ?, ?)
ON CONFLICT(project_id, chat_id) DO UPDATE SET session_id = excluded.session_id
WHERE chat_legacy_alias.session_id IS NOT excluded.session_id`, project, session.ID, key); err != nil {
		return err
	}
	return tx.Commit()
}

type messageMetadata struct {
	Message Message            `json:"message"`
	Images  []storedAttachment `json:"images,omitempty"`
	Media   []storedAttachment `json:"media,omitempty"`
}

func loadChat(project, id string) (*Session, bool, error) {
	db, home, err := openChatDatabase()
	if err != nil {
		return nil, false, err
	}
	defer db.Close()
	return readStoredChat(db.DB, home, project, id)
}

func readStoredChat(db *sql.DB, home, project, id string) (*Session, bool, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var key, metadata, currentID string
	var version int
	err = tx.QueryRow(`
SELECT c.sha256, c.metadata, c.chat_id, c.format_version
FROM chat_session c JOIN chat_legacy_alias a ON a.session_id = c.sha256
WHERE a.project_id = ? AND a.chat_id = ?`, project, id).Scan(&key, &metadata, &currentID, &version)
	if errors.Is(err, sql.ErrNoRows) {
		var known bool
		err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_legacy_alias WHERE project_id = ? AND chat_id = ?)", project, id).Scan(&known)
		return nil, known, err
	}
	if err != nil {
		return nil, false, err
	}
	if version != chatFormatVersion {
		return nil, true, fmt.Errorf("unsupported chat format version %d", version)
	}
	var session Session
	if err := json.Unmarshal([]byte(metadata), &session); err != nil {
		return nil, true, err
	}
	session.ID = currentID
	streams := make(map[string]*[]Message)
	for _, stream := range streamsFor(&session) {
		streams[stream.name] = stream.messages
	}
	rows, err := tx.Query(`
SELECT membership.stream, message.role, message.content, message.metadata
FROM chat_membership membership JOIN chat_message message ON message.sha256 = membership.message_id
WHERE membership.session_id = ? ORDER BY membership.stream, membership.position`, key)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	for rows.Next() {
		var stream, role, content, raw string
		if err := rows.Scan(&stream, &role, &content, &raw); err != nil {
			return nil, true, err
		}
		var metadata messageMetadata
		if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
			return nil, true, err
		}
		message := metadata.Message
		message.Role, message.Content = role, content
		message.ToolImages, err = restoreAttachments(home, metadata.Images)
		if err != nil {
			return nil, true, err
		}
		message.MediaPreviews, err = restoreAttachments(home, metadata.Media)
		if err != nil {
			return nil, true, err
		}
		target, ok := streams[stream]
		if !ok {
			return nil, true, fmt.Errorf("unknown stream %q", stream)
		}
		*target = append(*target, message)
	}
	return &session, true, rows.Err()
}

func renameStoredChat(project, oldID, newID string) (bool, error) {
	db, _, err := openChatDatabase()
	if err != nil {
		return false, err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var key string
	err = tx.QueryRow("SELECT sha256 FROM chat_session WHERE project_id = ? AND chat_id = ?", project, oldID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec("UPDATE chat_session SET chat_id = ? WHERE sha256 = ?", newID, key); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`
INSERT INTO chat_legacy_alias(project_id, chat_id, session_id) VALUES (?, ?, ?)
ON CONFLICT(project_id, chat_id) DO UPDATE SET session_id = excluded.session_id`, project, newID, key); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func removeStoredChat(project, id string) error {
	db, _, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM chat_session WHERE project_id = ? AND chat_id = ?", project, id); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO chat_legacy_alias(project_id, chat_id) VALUES (?, ?)", project, id); err != nil {
		return err
	}
	return tx.Commit()
}

func storedChatIndex(project string) (map[string]bool, []*Session, error) {
	db, _, err := openChatDatabase()
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	rows, err := db.Query(`
SELECT a.chat_id, c.chat_id, c.title, c.epoch_ns, c.last_activity, c.last_user_activity
FROM chat_legacy_alias a LEFT JOIN chat_session c ON c.sha256 = a.session_id WHERE a.project_id = ?`, project)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	known := make(map[string]bool)
	var sessions []*Session
	for rows.Next() {
		var alias string
		var id, title, last, lastUser sql.NullString
		var epoch sql.NullInt64
		if err := rows.Scan(&alias, &id, &title, &epoch, &last, &lastUser); err != nil {
			return nil, nil, err
		}
		known[alias] = true
		if !id.Valid || id.String != alias {
			continue
		}
		lastAt, _ := time.Parse(time.RFC3339Nano, last.String)
		lastUserAt, _ := time.Parse(time.RFC3339Nano, lastUser.String)
		createdAt := time.Time{}
		if epoch.Int64 != (time.Time{}).UnixNano() {
			createdAt = time.Unix(0, epoch.Int64).UTC()
		}
		sessions = append(sessions, &Session{ID: id.String, Title: title.String, CreatedAt: createdAt, LastMessageAt: lastAt, LastUserMessageAt: lastUserAt})
	}
	return known, sessions, rows.Err()
}

func readLegacyChat(path string) (*Session, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func summaryLastUserActivity(session *Session) time.Time {
	latest := session.LastUserMessageAt
	for _, message := range session.Messages {
		if message.Role == "user" && message.CreatedAt.After(latest) {
			latest = message.CreatedAt
		}
	}
	return latest
}
