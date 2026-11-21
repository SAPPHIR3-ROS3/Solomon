package database

import (
	"context"
	"database/sql"
)

func migrateSidebarOrder(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
ALTER TABLE chat_session ADD COLUMN sidebar_activity_ns INTEGER NOT NULL DEFAULT 0;
UPDATE chat_session SET sidebar_activity_ns = CASE
    WHEN last_user_activity = '0001-01-01T00:00:00Z' THEN epoch_ns
    ELSE CAST(strftime('%s', substr(last_user_activity, 1, 19)) AS INTEGER) * 1000000000
         + CAST(substr(substr(rtrim(last_user_activity, 'Z'), 21) || '000000000', 1, 9) AS INTEGER)
END;
CREATE INDEX chat_session_sidebar ON chat_session(project_id, sidebar_activity_ns DESC, chat_id);
`); err != nil {
		return err
	}
	return nil
}
