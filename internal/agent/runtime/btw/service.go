package btw

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/btw/listener"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type request struct {
	question        string
	assistantPrefix string
}

type Service struct {
	host    Host
	mux     *OutputMux
	runCtx  context.Context
	stopRun func(error)
	stopErr error

	mu       sync.Mutex
	queue    []request
	active   bool
	pending  string
	wake     chan struct{}
	stop     chan struct{}
	wg       sync.WaitGroup
	listener *listener.Listener
}

func NewService(host Host, mux *OutputMux, runCtx context.Context, stopRun func(error), stopErr error) *Service {
	return &Service{
		host:    host,
		mux:     mux,
		runCtx:  runCtx,
		stopRun: stopRun,
		stopErr: stopErr,
		wake:    make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
}

func (s *Service) Start() {
	if s == nil || s.mux == nil {
		return
	}
	if listener.Available() {
		s.listener = listener.New(s.beginCollect, s.enqueue, s.discardCollect, s.triggerStop)
		s.listener.Start()
	}
	s.wg.Add(1)
	go s.worker()
}

func (s *Service) Stop() {
	if s == nil {
		return
	}
	close(s.stop)
	if s.listener != nil {
		s.listener.Stop()
	}
	s.wg.Wait()
}

func (s *Service) enqueue(question string) {
	s.mu.Lock()
	s.queue = append(s.queue, request{question: question, assistantPrefix: s.pending})
	s.pending = ""
	s.mu.Unlock()
	if s.mux != nil {
		s.mux.SetBufferMain()
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) beginCollect() (string, bool) {
	if s.mux == nil {
		return "", false
	}
	s.mux.SetBufferMain()
	userPrefix, assistantPrefix := s.host.BtwLinePrefixes()
	s.mu.Lock()
	s.pending = assistantPrefix
	s.mu.Unlock()
	out := s.mux.Live()
	_, _ = io.WriteString(out, "\n")
	termcolor.PrintBtwSeparator(out)
	termcolor.WriteSystem(out, "/btw is temporary: this message and its answer will not be saved, shown in transcripts, or restored when the chat is reloaded. If this was accidental, delete the line to resume the main stream.")
	if f, ok := out.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	line, err := s.host.ReadBtwInput(out, userPrefix, "/btw ")
	if err != nil {
		return "", false
	}
	return line, true
}

func (s *Service) discardCollect() {
	if s.mux == nil || !s.canFlushDiscard() {
		return
	}
	s.mu.Lock()
	s.pending = ""
	s.mu.Unlock()
	s.mux.FlushBurst()
}

func (s *Service) canFlushDiscard() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue) == 0 && !s.active
}

func (s *Service) triggerStop() {
	s.mu.Lock()
	s.queue = nil
	s.pending = ""
	s.mu.Unlock()
}

func (s *Service) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
			s.drainQueue()
		case <-s.runCtx.Done():
			if s.mux != nil && s.mux.Buffering() {
				s.mux.FlushBurst()
			}
			return
		}
	}
}

func (s *Service) drainQueue() {
	for {
		req := s.popOne()
		if req.question == "" {
			return
		}
		s.setActive(true)
		if !s.mux.Buffering() {
			s.mux.SetBufferMain()
		}
		ctx := context.WithoutCancel(s.runCtx)
		_ = Execute(ctx, s.host, s.mux.Live(), req.question, req.assistantPrefix)
		if !s.queueEmpty() {
			continue
		}
		if s.waitCatchUpForMore() {
			continue
		}
		s.mux.FlushBurst()
		s.setActive(false)
		return
	}
}

func (s *Service) setActive(active bool) {
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
}

func (s *Service) popOne() request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return request{}
	}
	q := s.queue[0]
	s.queue = s.queue[1:]
	return q
}

func (s *Service) queueEmpty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue) == 0
}

func (s *Service) waitCatchUpForMore() bool {
	timer := time.NewTimer(time.Duration(CatchUpPause) * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return false
		case <-s.wake:
			if !s.queueEmpty() {
				return true
			}
		case <-s.stop:
			return false
		}
	}
}
