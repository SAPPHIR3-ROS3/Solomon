package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type LogEvent struct {
	Time           time.Time
	Source         string
	Level          string
	Message        string
	CorrelationKey string
}

func AppendLog(ctx context.Context, db *sql.DB, event LogEvent) (string, error) {
	epoch := event.Time.UnixNano()
	day := event.Time.Format("2006-01-02")
	canonical, err := json.Marshal([]any{epoch, day, event.Source, event.Level, event.Message, event.CorrelationKey})
	if err != nil {
		return "", err
	}
	id := digest(canonical)
	_, err = db.ExecContext(ctx, `
INSERT INTO log_event(sha256, epoch_ns, day, source, level, message, correlation_key)
VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(sha256) DO NOTHING`,
		id, epoch, day, event.Source, event.Level, event.Message, event.CorrelationKey)
	return id, err
}
