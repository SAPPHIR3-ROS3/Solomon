package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Resource *struct {
		URI      string `json:"uri"`
		Text     string `json:"text"`
		Blob     string `json:"blob"`
		MimeType string `json:"mimeType"`
	} `json:"resource,omitempty"`
}
type RichRunner interface {
	PromptContent(context.Context, []Content, func(Update)) error
}
type StateRunner interface {
	Snapshot() (json.RawMessage, error)
	Restore(json.RawMessage) error
}
type ConfigRunner interface {
	Configure(context.Context, string, string) error
	Configuration() map[string]any
}
type PermissionRunner interface {
	SetPermissionHandler(func(context.Context, Update) error)
}
type record struct {
	ID          string          `json:"sessionId"`
	CWD         string          `json:"cwd"`
	Title       string          `json:"title"`
	UpdatedAt   string          `json:"updatedAt"`
	State       json.RawMessage `json:"state,omitempty"`
	History     []Update        `json:"history"`
	Mode        string          `json:"mode"`
	Permissions map[string]bool `json:"permissions,omitempty"`
}

func (s *Server) SetStorage(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	s.storage = dir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var r record
		if err = json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("read ACP session %s: %w", e.Name(), err)
		}
		if validID(r.ID) {
			s.sessions[r.ID] = &session{cwd: r.CWD, record: r}
		}
	}
	return nil
}
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func (s *Server) save(v *session) error {
	if s.storage == "" {
		return nil
	}
	data, err := json.Marshal(v.record)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.storage, ".session-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.storage, v.record.ID+".json"))
}
func modes(mode string) map[string]any {
	if mode != "chat" {
		mode = "code"
	}
	return map[string]any{"currentModeId": mode, "availableModes": []any{map[string]any{"id": "code", "name": "Code", "description": "Use tools with Solomon execution policies"}, map[string]any{"id": "chat", "name": "Chat", "description": "Conversation without agent tools"}}}
}
func (s *Server) sessionResult(v *session, includeID bool) map[string]any {
	result := map[string]any{"modes": modes(v.record.Mode)}
	if includeID {
		result["sessionId"] = v.record.ID
	}
	if r, ok := v.runner.(ConfigRunner); ok {
		for k, val := range r.Configuration() {
			result[k] = val
		}
	}
	if options, ok := result["configOptions"]; ok {
		result["configOptions"] = modeOptions(v.record.Mode, options)
	}
	return result
}
func modeOptions(mode string, options any) []any {
	if mode != "chat" {
		mode = "code"
	}
	opts, _ := options.([]any)
	return append([]any{map[string]any{"id": "mode", "name": "Mode", "category": "mode", "type": "select", "currentValue": mode, "options": []any{map[string]any{"value": "code", "name": "Code"}, map[string]any{"value": "chat", "name": "Chat"}}}}, opts...)
}
func (s *Server) extended(ctx context.Context, r request, p params) bool {
	if err := s.discover(p.SessionID); err != nil {
		s.failure(r.ID, err)
		return true
	}
	switch r.Method {
	case "session/list":
		if p.CWD != "" && !filepath.IsAbs(p.CWD) {
			s.reply(r.ID, nil, -32602, "cwd filter must be absolute")
			return true
		}
		if err := s.refresh(); err != nil {
			s.failure(r.ID, err)
			return true
		}
		ids := make([]string, 0, len(s.sessions))
		for id, v := range s.sessions {
			if p.CWD == "" || v.cwd == p.CWD {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		start := 0
		if p.Cursor != "" {
			start = sort.SearchStrings(ids, p.Cursor)
			if start < len(ids) && ids[start] == p.Cursor {
				start++
			}
		}
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		rows := []any{}
		for _, id := range ids[start:end] {
			v := s.sessions[id]
			rows = append(rows, map[string]any{"sessionId": id, "cwd": v.cwd, "title": v.record.Title, "updatedAt": v.record.UpdatedAt})
		}
		result := map[string]any{"sessions": rows}
		if end < len(ids) {
			result["nextCursor"] = ids[end-1]
		}
		s.reply(r.ID, result, 0, "")
		return true
	case "session/load", "session/resume":
		if _, err := ParseMCP(p.MCPServers); err != nil {
			s.reply(r.ID, nil, -32602, err.Error())
			return true
		}
		v := s.sessions[p.SessionID]
		if v == nil {
			s.reply(r.ID, nil, -32002, "Session not found")
			return true
		}
		if v.cancel != nil || v.closing {
			s.reply(r.ID, nil, -32600, "Session is busy")
			return true
		}
		if err := s.activate(v); err != nil {
			s.reply(r.ID, nil, -32600, err.Error())
			return true
		}
		if p.CWD != v.cwd {
			s.reply(r.ID, nil, -32602, "cwd does not match session")
			return true
		}
		if v.resourceCancel != nil {
			v.resourceCancel()
		}
		if v.runner != nil {
			_ = v.runner.Close()
			v.runner = nil
		}
		v.servers = p.MCPServers
		s.prepare(ctx, r, v, false, r.Method == "session/load")
		return true
	case "session/close":
		v := s.sessions[p.SessionID]
		if v == nil {
			s.reply(r.ID, map[string]any{}, 0, "")
			return true
		}
		if v.closing {
			s.reply(r.ID, nil, -32600, "Session is closing")
			return true
		}
		v.closing = true
		if v.cancel != nil {
			v.cancel()
		}
		done := v.done
		s.workers.Add(1)
		go func() {
			defer s.workers.Done()
			if done != nil {
				<-done
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if v.resourceCancel != nil {
				v.resourceCancel()
			}
			if v.runner != nil {
				_ = v.runner.Close()
				v.runner = nil
			}
			v.release()
			v.closing = false
			s.reply(r.ID, map[string]any{}, 0, "")
		}()
		return true
	case "session/delete":
		v := s.sessions[p.SessionID]
		if v != nil {
			if v.cancel != nil || v.closing {
				s.reply(r.ID, nil, -32600, "Session is busy")
				return true
			}
			if err := s.activate(v); err != nil {
				s.reply(r.ID, nil, -32600, err.Error())
				return true
			}
			if s.storage != "" {
				err := os.Remove(filepath.Join(s.storage, v.record.ID+".json"))
				if err != nil && !os.IsNotExist(err) {
					s.reply(r.ID, nil, -32603, err.Error())
					return true
				}
			}
			if v.resourceCancel != nil {
				v.resourceCancel()
			}
			if v.runner != nil {
				_ = v.runner.Close()
			}
			v.release()
			delete(s.sessions, p.SessionID)
		}
		s.reply(r.ID, map[string]any{}, 0, "")
		return true
	case "session/set_mode", "session/set_config_option", "session/set_model":
		v := s.sessions[p.SessionID]
		if v == nil {
			s.reply(r.ID, nil, -32002, "Session not found")
			return true
		}
		if v.cancel != nil || v.closing {
			s.reply(r.ID, nil, -32600, "Session is busy")
			return true
		}
		if err := s.activate(v); err != nil {
			s.reply(r.ID, nil, -32600, err.Error())
			return true
		}
		key, value := p.ConfigID, p.Value
		if r.Method == "session/set_mode" {
			key, value = "mode", p.ModeID
		}
		if r.Method == "session/set_model" {
			key, value = "model", p.ModelID
		}
		if key == "mode" {
			if value != "code" && value != "chat" {
				s.reply(r.ID, nil, -32602, "Unknown mode")
				return true
			}
			v.record.Mode = value
			if err := s.save(v); err != nil {
				s.reply(r.ID, nil, -32603, err.Error())
				return true
			}
			s.notify(p.SessionID, Update{"sessionUpdate": "current_mode_update", "currentModeId": value})
			if opts, ok := s.sessionResult(v, false)["configOptions"]; ok {
				s.notify(p.SessionID, Update{"sessionUpdate": "config_option_update", "configOptions": opts})
			}
			result := map[string]any{}
			if r.Method == "session/set_config_option" {
				result = s.sessionResult(v, false)
			}
			s.reply(r.ID, result, 0, "")
			return true
		}
		s.workers.Add(1)
		turn, cancel := context.WithCancel(ctx)
		v.cancel = cancel
		v.operationID = r.ID
		v.done = make(chan struct{})
		go func() {
			defer s.workers.Done()
			defer cancel()
			err := s.withRunner(ctx, turn, v, func() error {
				cr, ok := v.runner.(ConfigRunner)
				if !ok {
					return fmt.Errorf("session configuration unavailable")
				}
				if err := cr.Configure(turn, key, value); err != nil {
					return err
				}
				if sr, ok := v.runner.(StateRunner); ok {
					state, err := sr.Snapshot()
					if err != nil {
						return err
					}
					v.record.State = state
				}
				return nil
			})
			s.mu.Lock()
			defer s.mu.Unlock()
			v.cancel = nil
			close(v.done)
			if err != nil {
				s.reply(r.ID, nil, -32602, err.Error())
				return
			}
			if err := s.save(v); err != nil {
				s.failure(r.ID, err)
				return
			}
			result := s.sessionResult(v, false)
			if opts, ok := result["configOptions"]; ok {
				s.notify(p.SessionID, Update{"sessionUpdate": "config_option_update", "configOptions": opts})
			}
			s.reply(r.ID, result, 0, "")
		}()
		return true
	}
	return false
}
func (s *Server) notify(id string, u Update) {
	s.send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": id, "update": u}})
}
func (s *Server) stamp(v *session) { v.record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano) }
