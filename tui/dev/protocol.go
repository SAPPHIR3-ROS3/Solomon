package dev

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Request struct {
	Token string `json:"token"`
}
type Status struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Generation int    `json:"generation"`
}

func atomicJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
func nonce() (string, error) {
	var b [24]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func verifyState(path, token string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state struct {
		Version int             `json:"version"`
		Token   string          `json:"reload_token"`
		Data    json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Version != 1 || token == "" || state.Token != token || len(state.Data) == 0 {
		return errors.New("invalid handoff state token or envelope")
	}
	return os.Chmod(path, 0600)
}
