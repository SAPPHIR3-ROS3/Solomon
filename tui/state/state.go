package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const Version = 1
const MaxBytes = 16 << 20

type Envelope struct {
	Version     int             `json:"version"`
	ReloadToken string          `json:"reload_token,omitempty"`
	SavedAt     time.Time       `json:"saved_at"`
	Data        json.RawMessage `json:"data"`
}

func Save(path, token string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data, err := json.Marshal(Envelope{Version: Version, ReloadToken: token, SavedAt: time.Now().UTC(), Data: payload})
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return fmt.Errorf("TUI state exceeds %d bytes", MaxBytes)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".tui-state-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func Load(path string, value any) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("TUI state exceeds %d bytes", MaxBytes)
	}
	var envelope Envelope
	if err = json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	if envelope.Version != Version {
		return "", fmt.Errorf("unsupported TUI state version %d", envelope.Version)
	}
	if err = json.Unmarshal(envelope.Data, value); err != nil {
		return "", err
	}
	return envelope.ReloadToken, nil
}
