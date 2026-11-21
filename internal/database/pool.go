package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Lease struct {
	*sql.DB
	entry *poolEntry
	once  sync.Once
}

type poolEntry struct {
	db      *sql.DB
	home    string
	file    os.FileInfo
	holders int
	retired bool
}

var dataPool struct {
	sync.Mutex
	current *poolEntry
}

func BorrowData(ctx context.Context, home string) (*Lease, error) {
	home, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	dataPool.Lock()
	defer dataPool.Unlock()

	path, err := Path(home, Data)
	if err != nil {
		return nil, err
	}
	info, statErr := os.Stat(path)
	entry := dataPool.current
	if entry == nil || entry.home != home || statErr != nil || !os.SameFile(entry.file, info) {
		db, err := Open(ctx, home, Data)
		if err != nil {
			return nil, err
		}
		info, err = os.Stat(path)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		db.SetMaxIdleConns(4)
		db.SetConnMaxIdleTime(time.Second)
		if entry != nil {
			entry.retired = true
			if entry.holders == 0 {
				_ = entry.db.Close()
			}
		}
		entry = &poolEntry{db: db, home: home, file: info}
		dataPool.current = entry
	}
	entry.holders++
	return &Lease{DB: entry.db, entry: entry}, nil
}

func (lease *Lease) Close() error {
	var err error
	lease.once.Do(func() {
		dataPool.Lock()
		defer dataPool.Unlock()
		entry := lease.entry
		entry.holders--
		if entry.retired && entry.holders == 0 {
			err = entry.db.Close()
		}
	})
	return err
}
