package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func StoreArtifact(home string, content []byte) (string, error) {
	id := digest(content)
	if err := storeArtifact(home, id, content); err != nil {
		return "", err
	}
	return id, nil
}

func storeArtifact(home, id string, content []byte) error {
	directory := filepath.Join(home, "media", "chat-snapshots")
	target := filepath.Join(directory, id)
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(content)
	syncErr := file.Sync()
	if err := errors.Join(writeErr, syncErr, file.Close()); err != nil {
		return err
	}
	if err := os.Link(file.Name(), target); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func ReadArtifact(home, id string) ([]byte, error) {
	expected, err := hex.DecodeString(id)
	if err != nil || len(expected) != sha256.Size {
		return nil, fmt.Errorf("invalid artifact digest")
	}
	content, err := os.ReadFile(filepath.Join(home, "media", "chat-snapshots", id))
	if err != nil {
		return nil, err
	}
	if digest(content) != id {
		return nil, fmt.Errorf("artifact checksum mismatch")
	}
	return content, nil
}
