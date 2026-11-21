package chatstore

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

func sidebarActivity(session *Session) time.Time {
	sent := summaryLastUserActivity(session)
	if sent.IsZero() {
		return session.CreatedAt
	}
	return sent
}

func ListSidebarSummaries(project string, limit int) ([]*Session, error) {
	db, _, err := openChatDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := storedSidebar(tx, project, limit)
	if err != nil {
		return nil, err
	}
	legacy, err := legacySummaries(project, func() (map[string]bool, error) {
		return legacyChatAliases(tx, project)
	})
	if err != nil {
		return nil, err
	}
	result = append(result, legacy...)
	sort.Slice(result, func(i, j int) bool {
		left, right := sidebarActivity(result[i]), sidebarActivity(result[j])
		if left.Equal(right) {
			return result[i].ID < result[j].ID
		}
		return left.After(right)
	})
	if limit >= 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func storedSidebar(tx *sql.Tx, project string, limit int) ([]*Session, error) {
	rows, err := tx.Query(`
SELECT chat_id, title, epoch_ns, last_user_activity FROM chat_session
WHERE project_id = ? ORDER BY sidebar_activity_ns DESC, chat_id LIMIT ?`, project, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Session
	for rows.Next() {
		var v Session
		var epoch int64
		var last string
		if err := rows.Scan(&v.ID, &v.Title, &epoch, &last); err != nil {
			return nil, err
		}
		if epoch != (time.Time{}).UnixNano() {
			v.CreatedAt = time.Unix(0, epoch).UTC()
		}
		v.LastUserMessageAt, err = time.Parse(time.RFC3339Nano, last)
		if err != nil {
			return nil, err
		}
		result = append(result, &v)
	}
	return result, rows.Err()
}
