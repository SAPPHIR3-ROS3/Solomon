package acp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Update map[string]any
type Runner interface {
	Prompt(context.Context, string, func(Update)) error
	Close() error
}
type Factory func(context.Context, string, []json.RawMessage) (Runner, error)
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}
type session struct {
	servers        []json.RawMessage
	cwd            string
	runner         Runner
	cancel         context.CancelFunc
	record         record
	done           chan struct{}
	closing        bool
	resourceCancel context.CancelFunc
	operationID    json.RawMessage
	lock           *flock.Flock
}
type Server struct {
	out         io.Writer
	factory     Factory
	writeMu     sync.Mutex
	mu          sync.Mutex
	sessions    map[string]*session
	initialized bool
	workers     sync.WaitGroup
	storage     string
	pending     map[string]chan request
	nextID      uint64
	stop        context.CancelFunc
	outputErr   error
	version     string
}

func New(out io.Writer, factory Factory) *Server {
	return &Server{out: out, factory: factory, version: "development", sessions: map[string]*session{}, pending: map[string]chan request{}}
}
func (s *Server) send(v any) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.outputErr != nil {
		return
	}
	if err := json.NewEncoder(s.out).Encode(v); err != nil {
		s.outputErr = err
		if s.stop != nil {
			s.stop()
		}
	}
}
func (s *Server) reply(id json.RawMessage, result any, code int, message string) {
	if len(id) == 0 {
		return
	}
	m := map[string]any{"jsonrpc": "2.0", "id": id}
	if code != 0 {
		m["error"] = map[string]any{"code": code, "message": message}
	} else {
		m["result"] = result
	}
	s.send(m)
}
func (s *Server) Serve(ctx context.Context, in io.Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	s.stop = cancel
	stopReader := context.AfterFunc(ctx, func() {
		if c, ok := in.(io.Closer); ok {
			_ = c.Close()
		}
	})
	defer stopReader()
	defer func() {
		cancel()
		s.mu.Lock()
		for _, v := range s.sessions {
			if v.cancel != nil {
				v.cancel()
			}
		}
		s.mu.Unlock()
		s.workers.Wait()
		for _, v := range s.sessions {
			v.release()
			if v.runner != nil {
				_ = v.runner.Close()
			}
		}
	}()
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), 16*1024*1024)
	for scan.Scan() {
		var r request
		if !json.Valid(scan.Bytes()) {
			s.reply(json.RawMessage("null"), nil, -32700, "Parse error")
			continue
		}
		if json.Unmarshal(scan.Bytes(), &r) != nil {
			s.reply(json.RawMessage("null"), nil, -32600, "Invalid request")
			continue
		}
		if r.JSONRPC == "2.0" && r.Method == "" && len(r.ID) > 0 && (len(r.Result) > 0 || len(r.Error) > 0) {
			s.mu.Lock()
			ch := s.pending[string(r.ID)]
			s.mu.Unlock()
			if ch != nil {
				select {
				case ch <- r:
				default:
				}
			}
			continue
		}
		if r.JSONRPC != "2.0" || r.Method == "" {
			s.reply(json.RawMessage("null"), nil, -32600, "Invalid request")
			continue
		}
		if len(r.ID) > 0 {
			var id any
			if json.Unmarshal(r.ID, &id) != nil || id == nil {
				s.reply(json.RawMessage("null"), nil, -32600, "Invalid request ID")
				continue
			}
			switch id.(type) {
			case string:
			case float64:
				if strings.ContainsAny(string(r.ID), ".eE") {
					s.reply(json.RawMessage("null"), nil, -32600, "Request ID must be an integer or string")
					continue
				}
			default:
				s.reply(json.RawMessage("null"), nil, -32600, "Invalid request ID")
				continue
			}
		}
		if len(r.ID) == 0 && r.Method != "session/cancel" && r.Method != "$/cancel_request" {
			continue
		}
		s.handle(ctx, r)
	}
	s.writeMu.Lock()
	err := s.outputErr
	s.writeMu.Unlock()
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return scan.Err()
}
func (s *Server) handle(ctx context.Context, r request) {
	var p params
	if len(r.Params) > 0 && (strings.TrimSpace(string(r.Params))[0] != '{' || json.Unmarshal(r.Params, &p) != nil) {
		s.reply(r.ID, nil, -32602, "Invalid params")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == "initialize" {
		if p.ProtocolVersion < 1 {
			s.reply(r.ID, nil, -32602, "protocolVersion is required")
			return
		}
		s.initialized = true
		s.reply(r.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true, "promptCapabilities": map[string]any{"image": true, "embeddedContext": true}, "mcpCapabilities": map[string]any{"http": true, "sse": true}, "sessionCapabilities": map[string]any{"list": map[string]any{}, "resume": map[string]any{}, "delete": map[string]any{}, "close": map[string]any{}}}, "agentInfo": s.agentInfo(), "authMethods": []any{}}, 0, "")
		return
	}
	if !s.initialized {
		s.reply(r.ID, nil, -32600, "Initialize the connection first")
		return
	}
	if s.extended(ctx, r, p) {
		return
	}
	switch r.Method {
	case "session/new":
		if _, err := ParseMCP(p.MCPServers); err != nil {
			s.reply(r.ID, nil, -32602, err.Error())
			return
		}
		if !filepath.IsAbs(p.CWD) {
			s.reply(r.ID, nil, -32602, "cwd must be absolute")
			return
		}
		info, err := os.Stat(p.CWD)
		if err != nil || !info.IsDir() {
			s.reply(r.ID, nil, -32602, "cwd must be an existing directory")
			return
		}

		b := make([]byte, 16)
		if _, err = rand.Read(b); err != nil {
			s.reply(r.ID, nil, -32603, err.Error())
			return
		}
		id := hex.EncodeToString(b)
		v := &session{cwd: p.CWD, servers: p.MCPServers, record: record{ID: id, CWD: p.CWD, Mode: "code", History: []Update{}}}
		s.stamp(v)
		if err := s.save(v); err != nil {
			s.reply(r.ID, nil, -32603, err.Error())
			return
		}
		if err := s.activate(v); err != nil {
			s.failure(r.ID, err)
			return
		}
		s.sessions[id] = v
		s.prepare(ctx, r, v, true, false)
	case "$/cancel_request":
		for _, v := range s.sessions {
			if v.cancel != nil && string(v.operationID) == string(p.RequestID) {
				v.cancel()
			}
		}
	case "session/cancel":
		if v := s.sessions[p.SessionID]; v != nil && v.cancel != nil {
			v.cancel()
		}
	case "session/prompt":
		v := s.sessions[p.SessionID]
		if v == nil {
			s.reply(r.ID, nil, -32002, "Unknown session")
			return
		}
		if v.cancel != nil || v.closing {
			s.reply(r.ID, nil, -32600, "Session already has an active operation")
			return
		}
		if err := s.activate(v); err != nil {
			s.reply(r.ID, nil, -32600, err.Error())
			return
		}
		if err := ValidateContent(p.Prompt); err != nil {
			s.reply(r.ID, nil, -32602, err.Error())
			return
		}
		for _, block := range p.Prompt {
			v.record.History = append(v.record.History, Update{"sessionUpdate": "user_message_chunk", "content": block})
		}
		if v.record.Title == "" {
			for _, b := range p.Prompt {
				if b.Type == "text" {
					v.record.Title = b.Text
					if len(v.record.Title) > 120 {
						v.record.Title = v.record.Title[:120]
					}
					break
				}
			}
		}

		turn, cancel := context.WithCancel(ctx)
		v.cancel = cancel
		v.operationID = r.ID
		v.done = make(chan struct{})
		s.workers.Add(1)
		go func() {
			defer s.workers.Done()
			defer cancel()
			err := s.run(ctx, turn, v, p.SessionID, p.Prompt, func(u Update) {
				s.mu.Lock()
				if u["sessionUpdate"] == "config_option_update" {
					u["configOptions"] = modeOptions(v.record.Mode, u["configOptions"])
				}
				v.record.History = append(v.record.History, u)
				s.mu.Unlock()
				s.notify(p.SessionID, u)
			})
			s.mu.Lock()
			defer s.mu.Unlock()
			v.cancel = nil
			close(v.done)
			s.stamp(v)
			if persistErr := s.save(v); err == nil && persistErr != nil {
				err = persistErr
			}
			if turn.Err() != nil || errors.Is(err, context.Canceled) {
				s.reply(r.ID, map[string]any{"stopReason": "cancelled"}, 0, "")
			} else if err != nil {
				s.failure(r.ID, err)
			} else {
				reason := "end_turn"
				if runner, ok := v.runner.(interface{ StopReason() string }); ok {
					reason = runner.StopReason()
				}
				s.reply(r.ID, map[string]any{"stopReason": reason}, 0, "")
			}
		}()
	default:
		s.reply(r.ID, nil, -32601, fmt.Sprintf("Method not found: %s", r.Method))
	}
}

var execution = make(chan struct{}, 1)

func (s *Server) withRunner(lifetime, ctx context.Context, v *session, fn func() error) error {
	select {
	case execution <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-execution }()
	if err := ctx.Err(); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = os.Chdir(v.cwd); err != nil {
		return err
	}
	defer os.Chdir(cwd)
	if v.runner == nil {
		resources, cancel := context.WithCancel(lifetime)
		stopSetup := context.AfterFunc(ctx, cancel)
		runner, err := s.factory(resources, v.cwd, v.servers)
		if err != nil {
			stopSetup()
			cancel()
			return err
		}
		if len(v.record.State) > 0 {
			if sr, ok := runner.(interface {
				RestoreContext(context.Context, json.RawMessage) error
			}); ok {
				err = sr.RestoreContext(ctx, v.record.State)
			} else if sr, ok := runner.(StateRunner); ok {
				err = sr.Restore(v.record.State)
			}
		}
		stopSetup()
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			cancel()
			runner.Close()
			return err
		}
		v.resourceCancel = cancel
		v.runner = runner
	}
	return fn()
}
func (s *Server) run(lifetime, ctx context.Context, v *session, id string, blocks []Content, emit func(Update)) error {
	return s.withRunner(lifetime, ctx, v, func() error {
		if pr, ok := v.runner.(PermissionRunner); ok {
			pr.SetPermissionHandler(func(ctx context.Context, tool Update) error {
				return nil
			})
		}
		if v.record.Mode != "chat" {
			v.record.Mode = "code"
		}
		if cr, ok := v.runner.(ConfigRunner); ok {
			if err := cr.Configure(ctx, "mode", v.record.Mode); err != nil {
				return err
			}
		}
		var err error
		if rich, ok := v.runner.(RichRunner); ok {
			err = rich.PromptContent(ctx, blocks, emit)
		} else {
			var parts []string
			for _, b := range blocks {
				parts = append(parts, b.Text)
			}
			err = v.runner.Prompt(ctx, strings.Join(parts, "\n"), emit)
		}
		if sr, ok := v.runner.(StateRunner); ok {
			state, e := sr.Snapshot()
			if e != nil && err == nil {
				err = e
			}
			v.record.State = state
		}
		return err
	})
}
