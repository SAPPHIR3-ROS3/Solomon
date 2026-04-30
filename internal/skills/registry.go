package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"solomon/internal/logging"
)

func LoadRegistry(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewRegistry(), nil
		}
		return nil, err
	}
	return registryFromJSON(b)
}

func SaveRegistry(path string, r *Registry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func SaveMirrorJSON(path string, m map[string]SkillEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func WithRegistryLock(lockPath, registryPath string, fn func(*Registry) error) error {
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	fl := flock.New(lockPath)
	ok, err := fl.TryLockContext(context.Background(), 30*time.Second)
	if err != nil {
		return err
	}
	if !ok {
		logging.Log(logging.WARNING_LOG_LEVEL, "skills registry lock not acquired", logging.LogOptions{Params: map[string]any{"lock_path": lockPath}})
		return fmt.Errorf("could not acquire skills registry lock %s", lockPath)
	}
	defer fl.Unlock()

	reg, err := LoadRegistry(registryPath)
	if err != nil {
		return err
	}
	if err := fn(reg); err != nil {
		return err
	}
	return SaveRegistry(registryPath, reg)
}
