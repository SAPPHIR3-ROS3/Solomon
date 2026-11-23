package database

import (
	"context"
	"database/sql"
	"fmt"
)

const dataSchemaVersion = 11

const dataSchema = `
CREATE TABLE IF NOT EXISTS project_registration (
    root_path TEXT PRIMARY KEY,
    project_id TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS project_registration_project ON project_registration (project_id);
CREATE TABLE IF NOT EXISTS project_folder_state (
    project_id TEXT NOT NULL,
    panel TEXT NOT NULL,
    state TEXT NOT NULL,
    PRIMARY KEY (project_id, panel)
) STRICT;
CREATE TABLE IF NOT EXISTS project_pdf_preferences (
    project_id TEXT PRIMARY KEY,
    continuous INTEGER NOT NULL CHECK (continuous IN (0, 1)),
    direction TEXT NOT NULL CHECK (direction IN ('vertical', 'horizontal')),
    two_pages INTEGER NOT NULL CHECK (two_pages IN (0, 1)),
    zoom REAL NOT NULL CHECK (zoom BETWEEN 0.25 AND 3),
    fit_mode TEXT CHECK (fit_mode IN ('page', 'width'))
) STRICT;
CREATE TABLE IF NOT EXISTS project_file_position (
    project_id TEXT NOT NULL,
    path TEXT NOT NULL,
    view TEXT NOT NULL,
    position TEXT NOT NULL,
    PRIMARY KEY (project_id, path, view)
) STRICT;
CREATE TABLE IF NOT EXISTS legacy_import (
    source TEXT PRIMARY KEY
) STRICT;
CREATE TABLE IF NOT EXISTS chat_session (
    sha256 TEXT PRIMARY KEY CHECK (length(sha256) = 64),
    project_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    epoch_ns INTEGER NOT NULL,
    format_version INTEGER NOT NULL,
    title TEXT NOT NULL,
    last_activity TEXT NOT NULL,
    last_user_activity TEXT NOT NULL,
    metadata TEXT NOT NULL,
    UNIQUE (project_id, chat_id)
) STRICT;
CREATE TABLE IF NOT EXISTS chat_message (
    sha256 TEXT PRIMARY KEY CHECK (length(sha256) = 64),
    session_id TEXT NOT NULL REFERENCES chat_session(sha256) ON DELETE CASCADE,
    epoch_ns INTEGER,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS chat_message_session ON chat_message(session_id);
CREATE TABLE IF NOT EXISTS chat_membership (
    session_id TEXT NOT NULL REFERENCES chat_session(sha256) ON DELETE CASCADE,
    stream TEXT NOT NULL,
    position INTEGER NOT NULL,
    message_id TEXT NOT NULL REFERENCES chat_message(sha256) ON DELETE CASCADE,
    PRIMARY KEY (session_id, stream, position)
) STRICT;
CREATE TABLE IF NOT EXISTS chat_usage (
    session_id TEXT PRIMARY KEY REFERENCES chat_session(sha256) ON DELETE CASCADE,
    user_tokens INTEGER NOT NULL,
    reasoning_tokens INTEGER NOT NULL,
    response_tokens INTEGER NOT NULL
) STRICT;
CREATE TABLE IF NOT EXISTS chat_legacy_alias (
    project_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    session_id TEXT REFERENCES chat_session(sha256) ON DELETE SET NULL,
    PRIMARY KEY (project_id, chat_id)
) STRICT;
CREATE TABLE IF NOT EXISTS chat_import_failure (
    project_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    file_mtime_ns INTEGER NOT NULL,
    file_size INTEGER NOT NULL,
    description TEXT NOT NULL,
    PRIMARY KEY (project_id, chat_id)
) STRICT;
CREATE TABLE IF NOT EXISTS agent_session (
    storage_scope TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    sha256 TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    origin TEXT NOT NULL,
    parent_chat_id TEXT,
    parent_subchat_id TEXT,
    parent_tool_call_id TEXT,
    status TEXT NOT NULL,
    epoch_ns INTEGER NOT NULL,
    format_version INTEGER NOT NULL,
    metadata TEXT NOT NULL,
    PRIMARY KEY (storage_scope, agent_id)
) STRICT;
CREATE TABLE IF NOT EXISTS agent_message (
    session_id TEXT NOT NULL REFERENCES agent_session(sha256) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    message TEXT NOT NULL,
    PRIMARY KEY (session_id, position)
) STRICT;
CREATE TABLE IF NOT EXISTS agent_queue (
    session_id TEXT NOT NULL REFERENCES agent_session(sha256) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    position INTEGER NOT NULL,
    operation TEXT NOT NULL,
    PRIMARY KEY (session_id, kind, position)
) STRICT;
CREATE TABLE IF NOT EXISTS terminal_chat (
    terminal_id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    chat_id TEXT NOT NULL
) STRICT;
CREATE TABLE IF NOT EXISTS research_job (
    project_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    parent_chat_id TEXT,
    epoch_ns INTEGER,
    status TEXT,
    report_path TEXT,
    content TEXT,
    PRIMARY KEY (project_id, slug)
) STRICT;
CREATE TABLE IF NOT EXISTS acp_session (
    public_id TEXT PRIMARY KEY,
    cwd TEXT,
    updated_at TEXT,
    metadata TEXT
) STRICT;
CREATE TABLE IF NOT EXISTS acp_history (
    public_id TEXT NOT NULL REFERENCES acp_session(public_id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    event TEXT NOT NULL,
    PRIMARY KEY (public_id, position)
) STRICT;
CREATE TABLE IF NOT EXISTS managed_text (
    path TEXT PRIMARY KEY,
    content BLOB NOT NULL,
    content_sha256 TEXT NOT NULL CHECK (length(content_sha256) = 64)
) STRICT;
`

const logSchema = `
CREATE TABLE IF NOT EXISTS log_event (
    sha256 TEXT PRIMARY KEY CHECK (length(sha256) = 64),
    epoch_ns INTEGER NOT NULL,
    day TEXT NOT NULL,
    source TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    correlation_key TEXT NOT NULL
) STRICT;
`

func initialize(ctx context.Context, db *sql.DB, kind Kind) error {
	schema, id, current := dataSchema, 1397705796, dataSchemaVersion
	if kind == Logs {
		schema, id, current = logSchema, 1397705804, 7
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var databaseID, version int
	if err := tx.QueryRowContext(ctx, "PRAGMA application_id").Scan(&databaseID); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if databaseID == id && version == current {
		return tx.Commit()
	}
	if databaseID == 0 && version == 0 {
		var objects int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema").Scan(&objects); err != nil {
			return err
		}
		if objects != 0 {
			return fmt.Errorf("refusing to initialize a database containing %d existing schema objects", objects)
		}
	} else if databaseID != id || (version < 1 || version >= current) {
		return fmt.Errorf("unexpected database identity %d and schema version %d", databaseID, version)
	}
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if kind == Logs {
		if err := migrateLogDays(ctx, tx); err != nil {
			return err
		}
	}
	if kind == Data && version < 8 {
		if err := migrateSidebarOrder(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d; PRAGMA user_version = %d;", id, current)); err != nil {
		return err
	}
	return tx.Commit()
}

func migrateLogDays(ctx context.Context, tx *sql.Tx) error {
	var hasDay bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_table_info('log_event') WHERE name = 'day')").Scan(&hasDay); err != nil {
		return err
	}
	if !hasDay {
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE log_event ADD COLUMN day TEXT NOT NULL DEFAULT '';
UPDATE log_event SET day = date(epoch_ns / 1000000000, 'unixepoch', 'localtime');
`); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS log_event_source_day ON log_event (source, day, epoch_ns, sha256)")
	return err
}
