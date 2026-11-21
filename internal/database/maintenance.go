package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

func AcquireMaintenance(ctx context.Context, home string) (func() error, error) {
	return acquireStorageLock(ctx, home, false)
}

func acquireStorageLock(ctx context.Context, home string, shared bool) (func() error, error) {
	limit, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	directory := filepath.Join(home, "run")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(directory, "database.lock"), flock.SetPermissions(0o600))
	acquire := lock.TryLockContext
	if shared {
		acquire = lock.TryRLockContext
	}
	held, err := acquire(limit, 25*time.Millisecond)
	if err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("acquire Solomon database lock: %w", err)
	}
	if !held {
		_ = lock.Close()
		return nil, fmt.Errorf("Solomon database is in use")
	}
	return lock.Close, nil
}
