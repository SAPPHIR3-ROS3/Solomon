package acp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }
func (s *Server) failure(id json.RawMessage, err error) {
	code := -32603
	var rpc *Error
	if errors.Is(err, context.Canceled) {
		code = -32800
	}
	if errors.As(err, &rpc) {
		code = rpc.Code
	}
	s.reply(id, nil, code, err.Error())
}
func (s *Server) prepare(ctx context.Context, r request, v *session, includeID, replay bool) {
	turn, cancel := context.WithCancel(ctx)
	v.cancel = cancel
	v.operationID = r.ID
	v.done = make(chan struct{})
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer cancel()
		err := s.withRunner(ctx, turn, v, func() error { return nil })
		s.mu.Lock()
		defer s.mu.Unlock()
		v.cancel = nil
		close(v.done)
		if err != nil {
			if includeID {
				if s.storage != "" {
					_ = os.Remove(filepath.Join(s.storage, v.record.ID+".json"))
				}
				v.release()
				delete(s.sessions, v.record.ID)
			}
			s.failure(r.ID, err)
			return
		}
		if err = s.save(v); err != nil {
			s.failure(r.ID, err)
			return
		}
		if replay {
			for _, u := range v.record.History {
				s.notify(v.record.ID, u)
			}
		}
		s.reply(r.ID, s.sessionResult(v, includeID), 0, "")
		if runner, ok := v.runner.(interface{ Commands() []any }); ok {
			s.notify(v.record.ID, Update{"sessionUpdate": "available_commands_update", "availableCommands": runner.Commands()})
		}
	}()
}

func (s *Server) SetVersion(version string) { s.version = version }
