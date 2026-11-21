package logging

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

type processLogWriter struct {
	mu     sync.Mutex
	db     *sql.DB
	source string
	tail   []byte
	closed bool
}

func OpenProcessLog(home, source string) (io.WriteCloser, error) {
	db, err := database.Open(context.Background(), home, database.Logs)
	if err != nil {
		return nil, err
	}
	return &processLogWriter{db: db, source: source}, nil
}

func (w *processLogWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	const maxLine = 64 << 10
	original := len(data)
	for len(data) > 0 {
		index := bytes.IndexByte(data, '\n')
		length := len(data)
		if index >= 0 {
			length = index + 1
		}
		length = min(length, maxLine-len(w.tail))
		w.tail = append(w.tail, data[:length]...)
		data = data[length:]
		if len(w.tail) == maxLine || w.tail[len(w.tail)-1] == '\n' {
			w.flush()
		}
	}
	return original, nil
}

func (w *processLogWriter) flush() {
	if len(w.tail) == 0 {
		return
	}
	event := database.LogEvent{
		Time: time.Now(), Source: w.source, Level: "OUTPUT",
		Message: RedactReport(string(bytes.TrimSuffix(w.tail, []byte("\n")))),
	}
	if _, err := database.AppendLog(context.Background(), w.db, event); err != nil {
		_, _ = io.WriteString(os.Stderr, event.Message+"\n")
	}
	w.tail = w.tail[:0]
}

func (w *processLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	w.flush()
	_, trimErr := w.db.Exec(`
DELETE FROM log_event WHERE source = ? AND day IN (
    SELECT DISTINCT day FROM log_event WHERE source = ? ORDER BY day DESC LIMIT -1 OFFSET 7
)`, w.source, w.source)
	return errors.Join(trimErr, w.db.Close())
}
