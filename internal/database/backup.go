package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/ncruces/go-sqlite3/driver"
)

func Backup(ctx context.Context, db *sql.DB, destination string) (err error) {
	uri, err := fileURI(destination)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(destination))
		}
	}()
	if err = file.Close(); err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		connection, ok := raw.(driver.Conn)
		if !ok {
			return fmt.Errorf("backup requires a SQLite connection")
		}
		sqliteConn := connection.Raw()
		previous := sqliteConn.SetInterrupt(ctx)
		defer sqliteConn.SetInterrupt(previous)
		return sqliteConn.Backup("main", uri)
	})
}

func RestoreBackup(ctx context.Context, destination, source string) error {
	uri, err := fileURI(source)
	if err != nil {
		return err
	}
	target, err := fileURI(destination)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	db, err := driver.Open(target + "?mode=rw&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		connection, ok := raw.(driver.Conn)
		if !ok {
			return fmt.Errorf("restore requires a SQLite connection")
		}
		sqliteConn := connection.Raw()
		previous := sqliteConn.SetInterrupt(ctx)
		defer sqliteConn.SetInterrupt(previous)
		return sqliteConn.Restore("main", uri)
	})
}
