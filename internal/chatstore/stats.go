package chatstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const projectStatsVersion = 1

type chatUsageTotals struct {
	UserSum   int64 `json:"user_sum"`
	ReasonSum int64 `json:"reason_sum"`
	RespSum   int64 `json:"resp_sum"`
}

type projectStatsFile struct {
	Version int                        `json:"version"`
	Chats   map[string]chatUsageTotals `json:"chats"`
}

func projectStatsPath(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "welcome_stats.json"), nil
}

func sessionUsageTotals(s *Session) chatUsageTotals {
	var out chatUsageTotals
	if s == nil {
		return out
	}
	for j, m := range s.Messages {
		if m.Role != "assistant" {
			continue
		}
		if assistantMessageHasStoredUsage(m) {
			out.UserSum += m.UserPromptTokens
			out.ReasonSum += m.ReasoningTokens
			out.RespSum += m.ResponseTokens
			continue
		}
		eu, er, es := estimateAssistantTurnTokens(s.Messages, j)
		out.UserSum += eu
		out.ReasonSum += er
		out.RespSum += es
	}
	return out
}

func loadProjectStatsFile(projectHex string) (*projectStatsFile, error) {
	p, err := projectStatsPath(projectHex)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f projectStatsFile
	if err := json.Unmarshal(b, &f); err != nil || f.Version != projectStatsVersion || f.Chats == nil {
		logging.Diagnostic("chat statistics cache rejected", map[string]any{"invalid_json": err != nil, "version": f.Version, "missing_chats": f.Chats == nil})
		return nil, nil
	}
	return &f, nil
}

func ProjectWelcomeStats(projectHex string) (chatCount int, userSum, reasonSum, respSum int64, err error) {
	db, home, err := openChatDatabase()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer db.Close()
	if err := backfillStoredUsage(db.DB, home, projectHex); err != nil {
		return 0, 0, 0, 0, err
	}
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`
SELECT COUNT(*), COALESCE(SUM(u.user_tokens), 0), COALESCE(SUM(u.reasoning_tokens), 0),
       COALESCE(SUM(u.response_tokens), 0)
FROM chat_session c JOIN chat_usage u ON u.session_id = c.sha256 WHERE c.project_id = ?`,
		projectHex).Scan(&chatCount, &userSum, &reasonSum, &respSum)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	legacy, err := legacyUsage(tx, projectHex)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, entry := range legacy {
		userSum += entry.UserSum
		reasonSum += entry.ReasonSum
		respSum += entry.RespSum
	}
	return chatCount + len(legacy), userSum, reasonSum, respSum, nil
}

func backfillStoredUsage(db *sql.DB, home, project string) error {
	rows, err := db.Query(`
SELECT c.chat_id, c.sha256 FROM chat_session c LEFT JOIN chat_usage u ON u.session_id = c.sha256
WHERE c.project_id = ? AND u.session_id IS NULL`, project)
	if err != nil {
		return err
	}
	defer rows.Close()
	missing := make(map[string]string)
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return err
		}
		missing[id] = key
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for id, key := range missing {
		session, _, err := readStoredChat(db, home, project, id)
		if err != nil || session == nil {
			continue
		}
		usage := sessionUsageTotals(session)
		if _, err := db.Exec(`
INSERT OR IGNORE INTO chat_usage(session_id, user_tokens, reasoning_tokens, response_tokens)
SELECT sha256, ?, ?, ? FROM chat_session WHERE sha256 = ?`,
			usage.UserSum, usage.ReasonSum, usage.RespSum, key); err != nil {
			return err
		}
	}
	return nil
}

func legacyUsage(tx *sql.Tx, project string) ([]chatUsageTotals, error) {
	directory, err := ChatsDir(project)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cache *projectStatsFile
	var known map[string]bool
	var totals []chatUsageTotals
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || id == entry.Name() {
			continue
		}
		if known == nil {
			known, err = legacyChatAliases(tx, project)
			if err != nil {
				return nil, err
			}
		}
		if known[id] {
			continue
		}
		if cache == nil {
			cache, _ = loadProjectStatsFile(project)
			if cache == nil {
				cache = &projectStatsFile{}
			}
		}
		if usage, ok := cache.Chats[id]; ok {
			totals = append(totals, usage)
			continue
		}
		session, err := readLegacyChat(filepath.Join(directory, entry.Name()))
		if err == nil && session.ID != "" {
			FinishSessionLoad(session)
			totals = append(totals, sessionUsageTotals(session))
		}
	}
	return totals, nil
}

func legacyChatAliases(tx *sql.Tx, project string) (map[string]bool, error) {
	rows, err := tx.Query("SELECT chat_id FROM chat_legacy_alias WHERE project_id = ?", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		known[id] = true
	}
	return known, rows.Err()
}
