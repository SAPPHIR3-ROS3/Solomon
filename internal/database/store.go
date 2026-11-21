package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

type Kind string

const (
	Data Kind = "data"
	Logs Kind = "logs"
)

func Path(home string, kind Kind) (string, error) {
	if home == "" || (kind != Data && kind != Logs) {
		return "", fmt.Errorf("invalid database location")
	}
	return filepath.Join(home, string(kind)+".db"), nil
}

func Open(ctx context.Context, home string, kind Kind) (*sql.DB, error) {
	path, err := Path(home, kind)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	var hooks []func(*sqlite3.Conn) error
	if kind == Data {
		release, err := acquireStorageLock(ctx, home, true)
		if err != nil {
			return nil, err
		}
		defer release()
		hooks = dataConnectionHooks(home)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}

	dsn, err := fileURI(path)
	if err != nil {
		return nil, err
	}
	db, err := driver.Open(dsn+"?mode=rw&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate", hooks...)
	if err != nil {
		return nil, err
	}
	if err := initialize(ctx, db, kind); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize %s database: %w", kind, err)
	}
	return db, nil
}

func dataConnectionHooks(home string) []func(*sqlite3.Conn) error {
	var leases sync.Map
	return []func(*sqlite3.Conn) error{
		func(connection *sqlite3.Conn) error {
			release, err := acquireStorageLock(context.Background(), home, true)
			if err != nil {
				return err
			}
			leases.Store(connection, release)
			return nil
		},
		func(connection *sqlite3.Conn) error {
			release, found := leases.LoadAndDelete(connection)
			if !found {
				return nil
			}
			return release.(func() error)()
		},
	}
}

func fileURI(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path = (&url.URL{Path: filepath.ToSlash(absolute)}).EscapedPath()
	if strings.HasPrefix(path, "//") {
		path = "%2F" + path[1:]
	}
	return "file:" + path, nil
}

func OpenReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	dsn, err := fileURI(path)
	if err != nil {
		return nil, err
	}
	db, err := driver.Open(dsn + "?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
