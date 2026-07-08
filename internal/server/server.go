package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type Options struct {
	Addr      string
	StaticDir string
	NoStatic  bool
	CertPath  string
	KeyPath   string
}

type Server struct {
	cfg      *config.Root
	prov     *config.Provider
	projHex  string
	projRoot string
	opts     Options
	hub      *Hub
	tokens   *tokenStore
	passkeys *passkeyStore
	http     *http.Server
	rt       *agentruntime.Runtime
}

func New(cfg *config.Root, prov *config.Provider, projHex, projRoot string, opts Options) (*Server, error) {
	tokens, err := loadTokenStore()
	if err != nil {
		return nil, err
	}
	passkeys, err := loadPasskeyStore()
	if err != nil {
		return nil, err
	}
	respStore, err := newResponseStore(projHex)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:      cfg,
		prov:     prov,
		projHex:  projHex,
		projRoot: projRoot,
		opts:     opts,
		hub:      NewHub(respStore),
		tokens:   tokens,
		passkeys: passkeys,
	}, nil
}

func (s *Server) Hub() *Hub {
	return s.hub
}

func (s *Server) Fingerprint() (string, error) {
	return ensureTLS(s.opts.CertPath, s.opts.KeyPath)
}

func (s *Server) BootstrapToken() (string, error) {
	return s.tokens.ensureBootstrap()
}

func (s *Server) ensureRuntime() (*agentruntime.Runtime, error) {
	if s.rt != nil {
		return s.rt, nil
	}
	sess := newConversationSession()
	rt := agentruntime.NewRuntime(nil, s.cfg, s.prov, s.projHex, s.projRoot, sess)
	rt.InitMCP(context.Background())
	s.rt = rt
	return rt, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("POST /v1/auth/bootstrap", s.handleBootstrap)
	mux.HandleFunc("POST /v1/auth/token", s.handleIssueToken)
	mux.HandleFunc("POST /v1/auth/token/revoke", s.requireAuth(s.handleRevokeToken))
	mux.HandleFunc("POST /v1/auth/passkey/register/begin", s.handlePasskeyRegisterBegin)
	mux.HandleFunc("POST /v1/auth/passkey/register/finish", s.handlePasskeyRegisterFinish)
	mux.HandleFunc("POST /v1/auth/passkey/login/begin", s.handlePasskeyLoginBegin)
	mux.HandleFunc("POST /v1/auth/passkey/login/finish", s.handlePasskeyLoginFinish)
	mux.HandleFunc("GET /v1/conversations", s.requireAuth(s.handleListConversations))
	mux.HandleFunc("POST /v1/conversations", s.requireAuth(s.handleCreateConversation))
	mux.HandleFunc("GET /v1/conversations/{id}", s.requireAuth(s.handleGetConversation))
	mux.HandleFunc("POST /v1/responses", s.requireAuth(s.handleCreateResponse))
	mux.HandleFunc("GET /v1/responses/{id}", s.requireAuth(s.handleGetResponse))
	mux.HandleFunc("POST /v1/responses/{id}/cancel", s.requireAuth(s.handleCancelResponse))
	if !s.opts.NoStatic {
		static := s.opts.StaticDir
		if static == "" {
			static = s.cfg.EffectiveServerStaticDir()
		}
		if static != "" {
			mux.Handle("/", staticHandler(static, mux))
		}
	}
	return mux
}

func (s *Server) ListenAndServe() error {
	fp, err := s.Fingerprint()
	if err != nil {
		return err
	}
	cert, key := s.opts.CertPath, s.opts.KeyPath
	if cert == "" || key == "" {
		cert, key = s.cfg.EffectiveServerTLSPaths()
	}
	if cert == "" || key == "" {
		home, err := paths.SolomonHome()
		if err != nil {
			return err
		}
		cert = filepath.Join(home, "server", "certs", "server.crt")
		key = filepath.Join(home, "server", "certs", "server.key")
	}
	if _, err := ensureTLS(cert, key); err != nil {
		return err
	}
	tlsCfg, err := loadTLSConfig(cert, key)
	if err != nil {
		return err
	}
	s.http = &http.Server{
		Addr:      s.opts.Addr,
		Handler:   s.Handler(),
		TLSConfig: tlsCfg,
	}
	fmt.Fprintf(os.Stderr, "solomon serve listening on https://%s (cert SHA256: %s)\n", s.opts.Addr, fp)
	return s.http.ListenAndServeTLS("", "")
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.rt != nil {
		_ = s.rt.Close()
	}
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, err := bearerFromAuthHeader(r.Header.Get("Authorization"))
		if err != nil || !s.tokens.valid(tok) {
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "invalid bearer token")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}
	session, id, err := s.tokens.consumeBootstrap(strings.TrimSpace(body.Token))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_bootstrap", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": session, "id": id})
}

func (s *Server) handleIssueToken(w http.ResponseWriter, r *http.Request) {
	auth, _ := bearerFromAuthHeader(r.Header.Get("Authorization"))
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	session, id, err := s.tokens.issue(body.Label, auth)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": session, "id": id})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	tok, err := bearerFromAuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_api_key", "invalid bearer token")
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	if body.ID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "id is required")
		return
	}
	if err := s.tokens.revokeSelf(tok, body.ID); err != nil {
		switch err.Error() {
		case "unauthorized":
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "invalid bearer token")
		case "can only revoke own token":
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case "token not found":
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": true, "id": body.ID})
}

func (s *Server) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if s.passkeys.hasCredentials() {
		auth, err := bearerFromAuthHeader(r.Header.Get("Authorization"))
		if err != nil || !s.tokens.valid(auth) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required to add passkeys")
			return
		}
	}
	creation, sessionID, err := s.passkeys.beginRegister(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"publicKey":  creation.Response,
		"mediation":  creation.Mediation,
	})
}

func (s *Server) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if s.passkeys.hasCredentials() {
		auth, err := bearerFromAuthHeader(r.Header.Get("Authorization"))
		if err != nil || !s.tokens.valid(auth) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required to add passkeys")
			return
		}
	}
	sessionID, cred, err := parsePasskeyFinishBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	first := !s.passkeys.hasCredentials()
	if err := s.passkeys.finishRegister(r, sessionID, cred); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_credential", err.Error())
		return
	}
	if !first {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if s.tokens.hasActiveTokens() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	tok, id, err := s.tokens.issuePasskeyLogin("passkey")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "id": id})
}

func (s *Server) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	assertion, sessionID, err := s.passkeys.beginLogin(r)
	if err != nil {
		if err.Error() == "no passkeys registered" {
			writeError(w, http.StatusBadRequest, "no_passkeys", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"publicKey":  assertion.Response,
		"mediation":  assertion.Mediation,
	})
}

func (s *Server) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	sessionID, cred, err := parsePasskeyFinishBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.passkeys.finishLogin(r, sessionID, cred); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credential", err.Error())
		return
	}
	tok, id, err := s.tokens.issuePasskeyLogin("passkey")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "id": id})
}

func (s *Server) handleListConversations(w http.ResponseWriter, _ *http.Request) {
	items, err := chatstore.ListRecent(s.projHex, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"id":         it.ID,
			"object":     "conversation",
			"title":      it.Title,
			"created_at": it.CreatedAt,
			"updated_at": it.LastMessageAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": out})
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, _ *http.Request) {
	sess := newConversationSession()
	id := newResponseID()
	sess.ID = id
	if err := chatstore.WriteSession(s.projHex, sess); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"object": "conversation",
	})
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := chatstore.ReadSession(s.projHex, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, conversationSnapshot(sess))
}

func conversationSnapshot(sess *chatstore.Session) map[string]any {
	return map[string]any{
		"id":         sess.ID,
		"object":     "conversation",
		"title":      sess.Title,
		"messages":   sess.Messages,
		"created_at": sess.CreatedAt,
		"updated_at": sess.LastMessageAt,
	}
}

func newResponseID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "resp_" + hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, typ, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"type": typ, "message": message},
	})
}

func (s *Server) loadSession(conversation string) (*chatstore.Session, error) {
	if conversation == "" {
		rt, err := s.ensureRuntime()
		if err != nil {
			return nil, err
		}
		return rt.Session, nil
	}
	sess, err := chatstore.ReadSession(s.projHex, conversation)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Server) slashDeps(rt *agentruntime.Runtime, out io.Writer) commands.Deps {
	return commands.Deps{
		Ctx:      context.Background(),
		Out:      out,
		Cfg:      s.cfg,
		ProjHex:  s.projHex,
		ProjRoot: s.projRoot,
		Session: func() *chatstore.Session {
			return rt.Session
		},
		SetSession: func(sess *chatstore.Session) {
			rt.Session = sess
		},
		GetMode: func() string { return rt.Mode },
		SetMode: func(m string) { rt.Mode = m },
		Model:   func() string { return rt.Model },
		Provider: func() *config.Provider {
			return s.prov
		},
	}
}

func isSlashInput(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "/")
}
