package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

type databaseSnapshot struct {
	home      string
	directory string
	files     *updater.InstallationBackup
	saved     map[database.Kind]string
}

func preserveStorage(ctx context.Context) (*databaseSnapshot, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	release, err := database.AcquireMaintenance(ctx, home)
	if err != nil {
		return nil, err
	}
	defer release()
	directory, err := os.MkdirTemp(home, ".update-storage-*")
	if err != nil {
		return nil, err
	}
	snapshot := &databaseSnapshot{home: home, directory: directory, saved: make(map[database.Kind]string)}
	for _, kind := range []database.Kind{database.Data, database.Logs} {
		if err := snapshot.preserveDatabase(ctx, kind); err != nil {
			snapshot.close()
			return nil, fmt.Errorf("back up %s database: %w", kind, err)
		}
	}
	snapshot.files, err = updater.BackupInstallation(
		filepath.Join(home, "config.toml"),
		filepath.Join(home, "mcp.json"),
		filepath.Join(home, "AGENTS.md"),
	)
	if err != nil {
		snapshot.close()
		return nil, err
	}
	return snapshot, nil
}

func (s *databaseSnapshot) preserveDatabase(ctx context.Context, kind database.Kind) error {
	path, err := database.Path(s.home, kind)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	destination := filepath.Join(s.directory, filepath.Base(path))
	if err := database.BackupExisting(ctx, path, destination); err != nil {
		return err
	}
	s.saved[kind] = destination
	return nil
}

func (s *databaseSnapshot) restore(ctx context.Context) error {
	release, err := database.AcquireMaintenance(ctx, s.home)
	if err != nil {
		return err
	}
	defer release()
	var failures []error
	for _, kind := range []database.Kind{database.Data, database.Logs} {
		path, err := database.Path(s.home, kind)
		if err == nil {
			if source := s.saved[kind]; source != "" {
				err = database.RestoreBackup(ctx, path, source)
			} else {
				err = os.Remove(path)
				if errors.Is(err, os.ErrNotExist) {
					err = nil
				}
			}
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("restore %s database: %w", kind, err))
		}
	}
	if err := s.files.Restore(); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (s *databaseSnapshot) close() {
	if s.files != nil {
		s.files.Close()
	}
	_ = os.RemoveAll(s.directory)
}
