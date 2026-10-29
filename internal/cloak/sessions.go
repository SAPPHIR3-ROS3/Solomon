package cloak

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Sessions struct {
	mu      sync.Mutex
	clients map[string]*Client
	create  func() (*Client, error)
	closed  bool
	stop    func() bool
}

func NewSessions(ctx context.Context, create func() (*Client, error)) *Sessions {
	if ctx == nil {
		ctx = context.Background()
	}
	if create == nil {
		create = func() (*Client, error) { return NewClient(Options{}) }
	}
	sessions := &Sessions{clients: make(map[string]*Client), create: create}
	sessions.mu.Lock()
	sessions.stop = context.AfterFunc(ctx, func() { _ = sessions.Close() })
	sessions.mu.Unlock()
	return sessions
}

func (s *Sessions) Get(projectID, chatID string) (*Client, error) {
	if projectID == "" || chatID == "" {
		return nil, fmt.Errorf("Cloak session requires project and chat IDs")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrUnavailable
	}
	key := projectID + "\x00" + chatID
	if client := s.clients[key]; client != nil {
		return client, nil
	}
	client, err := s.create()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrUnavailable
	}
	s.clients[key] = client
	return client, nil
}

func (s *Sessions) Remove(projectID, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := projectID + "\x00" + chatID
	client := s.clients[key]
	delete(s.clients, key)
	if client != nil {
		return client.Close()
	}
	return nil
}

func (s *Sessions) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.stop != nil {
		s.stop()
	}
	var failures []error
	for key, client := range s.clients {
		failures = append(failures, client.Close())
		delete(s.clients, key)
	}
	return errors.Join(failures...)
}
