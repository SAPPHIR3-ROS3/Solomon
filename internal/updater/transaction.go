package updater

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type installationSnapshot struct {
	target, saved string
	existed       bool
}

// InstallationBackup keeps independent copies until all new processes are ready.
// Failed restoration leaves its copies on disk and reports their locations.
type InstallationBackup struct {
	paths []installationSnapshot
}

func BackupInstallation(targets ...string) (*InstallationBackup, error) {
	backup := &InstallationBackup{}
	for _, target := range targets {
		entry := installationSnapshot{target: target}
		if _, err := os.Lstat(target); err == nil {
			entry.existed = true
			dir, err := os.MkdirTemp(filepath.Dir(target), ".solomon-rollback-*")
			if err != nil {
				backup.Close()
				return nil, err
			}
			entry.saved = filepath.Join(dir, "original")
			backup.paths = append(backup.paths, entry)
			if err := copyInstallationPath(target, entry.saved); err != nil {
				backup.Close()
				return nil, fmt.Errorf("preserve %s: %w", target, err)
			}
		} else if errors.Is(err, os.ErrNotExist) {
			backup.paths = append(backup.paths, entry)
		} else {
			backup.Close()
			return nil, err
		}
	}
	return backup, nil
}

func (b *InstallationBackup) Restore() error {
	var failures []error
	for i := range b.paths {
		entry := &b.paths[i]
		var err error
		if entry.existed {
			err = replaceInstallationPath(entry.saved, entry.target)
		} else {
			err = os.RemoveAll(entry.target)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("restore %s: %w; backup preserved at %s", entry.target, err, entry.saved))
			// Close must not discard the only remaining copy.
			entry.saved = ""
		}
	}
	return errors.Join(failures...)
}

func (b *InstallationBackup) Close() {
	for _, entry := range b.paths {
		if entry.saved != "" {
			_ = os.RemoveAll(filepath.Dir(entry.saved))
		}
	}
}

func (b *InstallationBackup) Locations() []string {
	var locations []string
	for _, entry := range b.paths {
		if entry.saved != "" {
			locations = append(locations, entry.saved)
		}
	}
	return locations
}

func CommitDirectoryInstall(staged, target string) error {
	info, err := os.Lstat(staged)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("staged installation is not a directory")
	}
	return replaceInstallationPath(staged, target)
}

func copyInstallationPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	}
	if info.IsDir() {
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyInstallationPath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported installation file: %s", source)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

func replaceInstallationPath(source, target string) error {
	if filepath.Clean(source) == filepath.Clean(target) {
		return fmt.Errorf("installation source equals destination")
	}
	if _, err := os.Lstat(source); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(filepath.Dir(target), ".solomon-replace-*")
	if err != nil {
		return err
	}
	previous := filepath.Join(directory, "previous")
	preserve := false
	defer func() {
		if !preserve {
			_ = os.RemoveAll(directory)
		}
	}()
	hadTarget := false
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, previous); err != nil {
			return fmt.Errorf("backup current installation: %w", err)
		}
		hadTarget = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		if hadTarget {
			if restoreErr := os.Rename(previous, target); restoreErr != nil {
				preserve = true
				return errors.Join(err, fmt.Errorf("restore failed: %w; original preserved at %s", restoreErr, previous))
			}
		}
		return fmt.Errorf("replace installation: %w", err)
	}
	return nil
}
