package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const passkeyUserName = "solomon"

type passkeyPersist struct {
	UserID      []byte                `json:"user_id"`
	Credentials []webauthn.Credential `json:"credentials"`
}

type passkeyStore struct {
	mu          sync.Mutex
	path        string
	userID      []byte
	credentials []webauthn.Credential
	sessions    map[string]webauthn.SessionData
}

func loadPasskeyStore() (*passkeyStore, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "server")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := &passkeyStore{
		path:     filepath.Join(dir, "passkeys.json"),
		sessions: map[string]webauthn.SessionData{},
	}
	b, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		if err := p.ensureUserID(); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var data passkeyPersist
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	p.userID = data.UserID
	p.credentials = data.Credentials
	if len(p.userID) == 0 {
		if err := p.ensureUserID(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *passkeyStore) ensureUserID() error {
	if len(p.userID) > 0 {
		return nil
	}
	var b [32]byte
	if _, err := randRead(b[:]); err != nil {
		return err
	}
	p.userID = b[:]
	return p.save()
}

func (p *passkeyStore) save() error {
	data := passkeyPersist{UserID: p.userID, Credentials: p.credentials}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.path)
}

func (p *passkeyStore) hasCredentials() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.credentials) > 0
}

func (p *passkeyStore) user() *passkeyUser {
	return &passkeyUser{id: p.userID, credentials: p.credentials}
}

func (p *passkeyStore) webAuthnFor(r *http.Request) (*webauthn.WebAuthn, error) {
	rpid, origin := rpFromRequest(r)
	return webauthn.New(&webauthn.Config{
		RPID:          rpid,
		RPDisplayName: "Solomon",
		RPOrigins:     []string{origin},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: 5 * time.Minute},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: 5 * time.Minute},
		},
	})
}

func rpFromRequest(r *http.Request) (rpid, origin string) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return host, scheme + "://" + r.Host
}

func (p *passkeyStore) beginRegister(r *http.Request) (*protocol.CredentialCreation, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, err := p.webAuthnFor(r)
	if err != nil {
		return nil, "", err
	}
	user := p.user()
	opts := []webauthn.RegistrationOption{}
	if len(p.credentials) > 0 {
		opts = append(opts, webauthn.WithExclusions(webauthn.Credentials(p.credentials).CredentialDescriptors()))
	}
	creation, session, err := w.BeginRegistration(user, opts...)
	if err != nil {
		return nil, "", err
	}
	p.sessions[session.Challenge] = *session
	return creation, session.Challenge, nil
}

func (p *passkeyStore) finishRegister(r *http.Request, sessionID string, credBody []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	session, ok := p.takeSession(sessionID)
	if !ok {
		return errors.New("session expired or missing")
	}
	w, err := p.webAuthnFor(r)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(credBody))
	if err != nil {
		return err
	}
	credential, err := w.CreateCredential(p.user(), session, parsed)
	if err != nil {
		return err
	}
	p.credentials = append(p.credentials, *credential)
	return p.save()
}

func (p *passkeyStore) beginLogin(r *http.Request) (*protocol.CredentialAssertion, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.credentials) == 0 {
		return nil, "", errors.New("no passkeys registered")
	}
	w, err := p.webAuthnFor(r)
	if err != nil {
		return nil, "", err
	}
	assertion, session, err := w.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", err
	}
	p.sessions[session.Challenge] = *session
	return assertion, session.Challenge, nil
}

func (p *passkeyStore) finishLogin(r *http.Request, sessionID string, credBody []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	session, ok := p.takeSession(sessionID)
	if !ok {
		return errors.New("session expired or missing")
	}
	w, err := p.webAuthnFor(r)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(credBody))
	if err != nil {
		return err
	}
	credential, err := w.ValidateDiscoverableLogin(p.discoverableUser, session, parsed)
	if err != nil {
		return err
	}
	p.replaceCredential(*credential)
	return p.save()
}

func (p *passkeyStore) takeSession(id string) (webauthn.SessionData, bool) {
	session, ok := p.sessions[id]
	if !ok {
		return webauthn.SessionData{}, false
	}
	delete(p.sessions, id)
	if !session.Expires.IsZero() && session.Expires.Before(time.Now()) {
		return webauthn.SessionData{}, false
	}
	return session, true
}

func (p *passkeyStore) discoverableUser(rawID, _ []byte) (webauthn.User, error) {
	for _, c := range p.credentials {
		if bytes.Equal(c.ID, rawID) {
			return p.user(), nil
		}
	}
	return nil, errors.New("unknown credential")
}

func (p *passkeyStore) replaceCredential(updated webauthn.Credential) {
	for i := range p.credentials {
		if bytes.Equal(p.credentials[i].ID, updated.ID) {
			p.credentials[i] = updated
			return
		}
	}
}

type passkeyUser struct {
	id          []byte
	credentials []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte {
	return u.id
}

func (u *passkeyUser) WebAuthnName() string {
	return passkeyUserName
}

func (u *passkeyUser) WebAuthnDisplayName() string {
	return "Solomon"
}

func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

type passkeyFinishBody struct {
	SessionID  string          `json:"session_id"`
	Credential json.RawMessage `json:"credential"`
}

func parsePasskeyFinishBody(r *http.Request) (sessionID string, cred []byte, err error) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil, err
	}
	var body passkeyFinishBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", nil, err
	}
	sessionID = body.SessionID
	if sessionID == "" {
		var legacy struct {
			Challenge string `json:"challenge"`
		}
		if json.Unmarshal(raw, &legacy) == nil && legacy.Challenge != "" {
			sessionID = legacy.Challenge
		}
	}
	if len(body.Credential) > 0 {
		cred = body.Credential
	} else {
		cred = raw
	}
	if sessionID == "" {
		return "", nil, errors.New("session_id required")
	}
	if len(cred) == 0 {
		return "", nil, errors.New("credential required")
	}
	return sessionID, cred, nil
}

func randRead(b []byte) (int, error) {
	return rand.Read(b)
}
