package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const tokenPrefix = "slm_"

type TokenRecord struct {
	ID        string    `json:"id"`
	Hash      string    `json:"hash"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked,omitempty"`
}

type tokenStore struct {
	mu        sync.Mutex
	path      string
	tokens    []TokenRecord
	bootstrap string
}

type tokenStoreFile struct {
	Tokens    []TokenRecord `json:"tokens"`
	Bootstrap string        `json:"bootstrap,omitempty"`
}

func loadTokenStore() (*tokenStore, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "server")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &tokenStore{path: filepath.Join(dir, "tokens.json")}
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var file tokenStoreFile
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	s.tokens = file.Tokens
	s.bootstrap = file.Bootstrap
	return s, nil
}

func (s *tokenStore) save() error {
	file := tokenStoreFile{Tokens: s.tokens, Bootstrap: s.bootstrap}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newRawToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return tokenPrefix + hex.EncodeToString(b[:]), nil
}

func (s *tokenStore) hasActiveTokensLocked() bool {
	for _, t := range s.tokens {
		if !t.Revoked {
			return true
		}
	}
	return false
}

func (s *tokenStore) ensureBootstrap() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bootstrap != "" {
		return s.bootstrap, nil
	}
	if s.hasActiveTokensLocked() {
		return "", errors.New("server already has session tokens; use an existing bearer token")
	}
	raw, err := newRawToken()
	if err != nil {
		return "", err
	}
	s.bootstrap = raw
	if err := s.save(); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *tokenStore) consumeBootstrap(raw string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bootstrap == "" || s.bootstrap != raw {
		return "", "", errors.New("invalid bootstrap token")
	}
	session, id, err := s.issueLocked("bootstrap", "")
	if err != nil {
		return "", "", err
	}
	s.bootstrap = ""
	if err := s.save(); err != nil {
		return "", "", err
	}
	return session, id, nil
}

func (s *tokenStore) issue(label string, authorizedBy string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if authorizedBy != "" && !s.validLocked(authorizedBy) {
		return "", "", errors.New("unauthorized")
	}
	if authorizedBy == "" && s.hasActiveTokensLocked() {
		return "", "", errors.New("existing tokens require authorization")
	}
	return s.issueLocked(label, authorizedBy)
}

func (s *tokenStore) issuePasskeyLogin(label string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issueLocked(label, "passkey")
}

func (s *tokenStore) issueLocked(label, _ string) (string, string, error) {
	raw, err := newRawToken()
	if err != nil {
		return "", "", err
	}
	id, err := newRawToken()
	if err != nil {
		return "", "", err
	}
	s.tokens = append(s.tokens, TokenRecord{
		ID:        id,
		Hash:      hashToken(raw),
		Label:     label,
		CreatedAt: time.Now().UTC(),
	})
	if err := s.save(); err != nil {
		return "", "", err
	}
	return raw, id, nil
}

func (s *tokenStore) valid(raw string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validLocked(raw)
}

func (s *tokenStore) validLocked(raw string) bool {
	h := hashToken(raw)
	for _, t := range s.tokens {
		if t.Revoked {
			continue
		}
		if t.Hash == h {
			return true
		}
	}
	return false
}

func (s *tokenStore) hasActiveTokens() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasActiveTokensLocked()
}

func (s *tokenStore) recordIDLocked(raw string) (string, bool) {
	h := hashToken(raw)
	for _, t := range s.tokens {
		if t.Revoked {
			continue
		}
		if t.Hash == h {
			return t.ID, true
		}
	}
	return "", false
}

func (s *tokenStore) revokeSelf(raw, targetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	selfID, ok := s.recordIDLocked(raw)
	if !ok {
		return errors.New("unauthorized")
	}
	if targetID != selfID {
		return errors.New("can only revoke own token")
	}
	for i := range s.tokens {
		if s.tokens[i].ID != targetID {
			continue
		}
		if s.tokens[i].Revoked {
			return nil
		}
		s.tokens[i].Revoked = true
		return s.save()
	}
	return errors.New("token not found")
}

func bearerFromAuthHeader(v string) (string, error) {
	const p = "Bearer "
	if len(v) < len(p) || v[:len(p)] != p {
		return "", errors.New("missing bearer token")
	}
	tok := v[len(p):]
	if tok == "" {
		return "", errors.New("empty bearer token")
	}
	return tok, nil
}

