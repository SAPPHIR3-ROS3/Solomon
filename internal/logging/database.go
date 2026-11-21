package logging

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func writeDatabaseLog(event database.LogEvent, retention int) error {
	ctx := context.Background()
	if _, err := database.AppendLog(ctx, globalLogDB, event); err != nil {
		return err
	}
	day := event.Time.Format("2006-01-02")
	if day == globalLogDay {
		return nil
	}
	_, err := globalLogDB.ExecContext(ctx, `
DELETE FROM log_event WHERE source = ? AND day IN (
    SELECT DISTINCT day FROM log_event WHERE source = ? ORDER BY day DESC LIMIT -1 OFFSET ?
)`, event.Source, event.Source, retention)
	if err == nil {
		globalLogDay = day
	}
	return err
}

func readDatabaseLogs(limit int) ([]string, error) {
	rows, err := globalLogDB.Query(`
SELECT message FROM log_event WHERE source = ? AND day = ?
ORDER BY epoch_ns DESC, sha256 DESC LIMIT ?`, "application", time.Now().Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return nil, err
		}
		entry := strings.Split(message, "\n")
		slices.Reverse(entry)
		lines = append(lines, entry...)
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	slices.Reverse(lines)
	return lines, rows.Err()
}
