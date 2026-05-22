package tooloutput

import (
	"encoding/json"
	"os"
	"sync"
)

type Meta struct {
	SessionID  string
	ToolCallID string
	ToolName   string
}

type Service struct {
	projectHex     string
	limits         Limits
	spillGenerated bool
	mu             sync.Mutex
}

func NewService(projectHex string, limits Limits) *Service {
	return &Service{projectHex: projectHex, limits: limits}
}

func (s *Service) Limits() Limits {
	if s == nil {
		return DefaultLimits()
	}
	return s.limits
}

func (s *Service) MarkSpillGenerated() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.spillGenerated = true
	s.mu.Unlock()
}

func (s *Service) SpillGenerated() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spillGenerated
}

func (s *Service) Close(othersActive bool) error {
	if s == nil {
		return nil
	}
	return CloseProjectTemp(s.projectHex, othersActive, s.SpillGenerated())
}

func (s *Service) Apply(v any, meta Meta) any {
	if s == nil || v == nil {
		return v
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	text := string(b)
	if !exceedsLimits(text, s.limits) {
		return v
	}
	spillPath, err := writeSpill(s.projectHex, meta.SessionID, meta.ToolCallID, meta.ToolName, b)
	if err != nil {
		return truncatedResult("", err)
	}
	s.MarkSpillGenerated()
	return truncatedResult(spillPath, nil)
}

func Startup(pid int) error {
	return RegisterInstance(pid)
}

func Shutdown(pid int, projectHex string, svc *Service) error {
	others := ActiveOtherInstances(pid)
	if svc != nil {
		_ = svc.Close(others > 0)
	} else if projectHex != "" {
		_ = CloseProjectTemp(projectHex, others > 0, false)
	}
	return UnregisterInstance(pid)
}

func CurrentPID() int {
	return os.Getpid()
}
