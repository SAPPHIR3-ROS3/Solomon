package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultTerminalCols  = 80
	defaultTerminalRows  = 24
	maxTerminalCols      = 1000
	maxTerminalRows      = 500
	terminalBufferBytes  = 512 << 10
	terminalBufferEvents = 2048
)

var (
	errTerminalSessionNotFound = errors.New("terminal session not found")
	errTerminalUnsupported     = errors.New("daemon PTY is not supported on this platform yet")
)

type terminalOutput struct {
	Data []byte
	Seq  uint64
}

type terminalSubscriber struct {
	ctx    context.Context
	Events chan terminalOutput
}

type terminalSession struct {
	id          string
	cwd         string
	process     terminalProcess
	done        chan struct{}
	finishOnce  sync.Once
	mu          sync.Mutex
	running     bool
	seq         uint64
	buffer      []terminalOutput
	bufferBytes int
	subscribers map[uint64]*terminalSubscriber
	nextSubID   uint64
}

type terminalManager struct {
	ctx      <-chan struct{}
	mu       sync.Mutex
	sessions map[string]*terminalSession
}

type terminalSessionInfo struct {
	Cwd     string `json:"cwd"`
	ID      string `json:"id"`
	Running bool   `json:"running"`
	Seq     uint64 `json:"seq"`
}

func newTerminalManager(ctx <-chan struct{}) *terminalManager {
	manager := &terminalManager{ctx: ctx, sessions: make(map[string]*terminalSession)}
	go func() {
		<-ctx
		manager.CloseAll()
	}()
	return manager
}

func (m *terminalManager) Create(requestedCwd string, cols, rows int) (*terminalSession, error) {
	cwd, err := terminalWorkingDirectory(requestedCwd)
	if err != nil {
		return nil, err
	}
	cols, rows = normalizeTerminalSize(cols, rows)
	shell, args, err := terminalShell()
	if err != nil {
		return nil, err
	}
	return m.create(cwd, cols, rows, shell, args)
}

// CreateTUI starts the normal Solomon REPL as a daemon-owned child process.
// The local CLI only transports keyboard/output bytes, so a dropped client
// connection does not terminate the REPL or its runtime.
func (m *terminalManager) CreateTUI(requestedCwd string, cols, rows int) (*terminalSession, error) {
	cwd, err := terminalWorkingDirectory(requestedCwd)
	if err != nil {
		return nil, err
	}
	cols, rows = normalizeTerminalSize(cols, rows)
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return m.create(cwd, cols, rows, executable, []string{"--daemon-tui", cwd})
}

func (m *terminalManager) create(cwd string, cols, rows int, shell string, args []string) (*terminalSession, error) {
	env := terminalEnvironment()
	process, err := startTerminalProcess(terminalProcessOptions{
		Args:  append([]string(nil), args...),
		Cols:  uint16(cols),
		Cwd:   cwd,
		Env:   env,
		Rows:  uint16(rows),
		Shell: shell,
	})
	if err != nil {
		return nil, fmt.Errorf("start terminal: %w", err)
	}
	session := &terminalSession{
		cwd:         cwd,
		done:        make(chan struct{}),
		id:          newTerminalID(),
		process:     process,
		running:     true,
		subscribers: make(map[uint64]*terminalSubscriber),
	}
	m.mu.Lock()
	m.sessions[session.id] = session
	m.mu.Unlock()
	go session.readOutput()
	return session, nil
}

func (m *terminalManager) Get(id string) (*terminalSession, error) {
	m.mu.Lock()
	session := m.sessions[id]
	m.mu.Unlock()
	if session == nil {
		return nil, errTerminalSessionNotFound
	}
	return session, nil
}

func (m *terminalManager) Close(id string) error {
	m.mu.Lock()
	session := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if session == nil {
		return errTerminalSessionNotFound
	}
	session.close()
	return nil
}

func (m *terminalManager) CloseAll() {
	m.mu.Lock()
	sessions := make([]*terminalSession, 0, len(m.sessions))
	for id, session := range m.sessions {
		delete(m.sessions, id)
		sessions = append(sessions, session)
	}
	m.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
}

func (s *terminalSession) Info() terminalSessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return terminalSessionInfo{Cwd: s.cwd, ID: s.id, Running: s.running, Seq: s.seq}
}

func (s *terminalSession) Subscribe(ctx context.Context, after uint64) ([]terminalOutput, <-chan terminalOutput, <-chan struct{}, func()) {
	s.mu.Lock()
	replay := make([]terminalOutput, 0, len(s.buffer))
	for _, output := range s.buffer {
		if output.Seq <= after {
			continue
		}
		replay = append(replay, cloneTerminalOutput(output))
	}
	id := s.nextSubID
	s.nextSubID++
	subscriber := &terminalSubscriber{ctx: ctx, Events: make(chan terminalOutput, 256)}
	s.subscribers[id] = subscriber
	done := s.done
	s.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.subscribers, id)
			s.mu.Unlock()
		})
	}
	return replay, subscriber.Events, done, cancel
}

func (s *terminalSession) Write(data []byte) error {
	s.mu.Lock()
	process := s.process
	running := s.running
	s.mu.Unlock()
	if !running {
		return errors.New("terminal session is not running")
	}
	_, err := process.Write(data)
	return err
}

func (s *terminalSession) Resize(cols, rows int) error {
	cols, rows = normalizeTerminalSize(cols, rows)
	s.mu.Lock()
	process := s.process
	running := s.running
	s.mu.Unlock()
	if !running {
		return errors.New("terminal session is not running")
	}
	return process.Resize(uint16(cols), uint16(rows))
}

func (s *terminalSession) close() {
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	_ = process.Kill()
	_ = process.Close()
}

func (s *terminalSession) readOutput() {
	buffer := make([]byte, 32<<10)
	for {
		count, err := s.process.Read(buffer)
		if count > 0 {
			s.emit(buffer[:count])
		}
		if err != nil {
			break
		}
	}
	_ = s.process.Wait()
	s.finish()
}

func (s *terminalSession) emit(data []byte) {
	value := append([]byte(nil), data...)
	s.mu.Lock()
	s.seq++
	output := terminalOutput{Data: value, Seq: s.seq}
	s.buffer = append(s.buffer, output)
	s.bufferBytes += len(value)
	for len(s.buffer) > terminalBufferEvents || s.bufferBytes > terminalBufferBytes {
		removed := s.buffer[0]
		s.buffer = s.buffer[1:]
		s.bufferBytes -= len(removed.Data)
	}
	subscribers := make([]*terminalSubscriber, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	for _, subscriber := range subscribers {
		select {
		case subscriber.Events <- cloneTerminalOutput(output):
		case <-subscriber.ctx.Done():
		}
	}
}

func (s *terminalSession) finish() {
	s.finishOnce.Do(func() {
		s.mu.Lock()
		s.running = false
		close(s.done)
		s.mu.Unlock()
	})
}

func cloneTerminalOutput(output terminalOutput) terminalOutput {
	return terminalOutput{Data: append([]byte(nil), output.Data...), Seq: output.Seq}
}

func terminalWorkingDirectory(requested string) (string, error) {
	value := strings.TrimSpace(requested)
	if value == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	candidate := value
	if !filepath.IsAbs(candidate) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		candidate = filepath.Join(home, candidate)
	}
	candidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("terminal cwd is not a directory: %s", candidate)
	}
	return candidate, nil
}

func terminalShell() (string, []string, error) {
	if runtime.GOOS == "windows" {
		return "", nil, errTerminalUnsupported
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-i"}, nil
}

func terminalEnvironment() []string {
	env := append([]string(nil), os.Environ()...)
	setTerminalEnvironment := func(key, value string) {
		prefix := key + "="
		for index, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[index] = prefix + value
				return
			}
		}
		env = append(env, prefix+value)
	}
	setTerminalEnvironment("TERM", "xterm-256color")
	setTerminalEnvironment("COLORTERM", "truecolor")
	return env
}

func normalizeTerminalSize(cols, rows int) (int, int) {
	if cols <= 0 {
		cols = defaultTerminalCols
	}
	if rows <= 0 {
		rows = defaultTerminalRows
	}
	return minInt(cols, maxTerminalCols), minInt(rows, maxTerminalRows)
}

func newTerminalID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "terminal-" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("terminal-%d", time.Now().UnixNano())
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
