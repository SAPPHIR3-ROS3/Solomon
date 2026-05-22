package tooloutput

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
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
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output marshal failed", logging.LogOptions{Params: map[string]any{"tool": meta.ToolName, "err": err.Error()}})
		return v
	}
	text := string(b)
	if !exceedsLimits(text, s.limits) {
		return v
	}
	spillPath, err := writeSpill(s.projectHex, meta.SessionID, meta.ToolCallID, meta.ToolName, b)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output spill failed", logging.LogOptions{Params: map[string]any{"tool": meta.ToolName, "session": meta.SessionID, "err": err.Error()}})
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
		if err := svc.Close(others > 0); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "tool output service close failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "err": err.Error()}})
		}
	} else if projectHex != "" {
		if err := CloseProjectTemp(projectHex, others > 0, false); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "tool output temp cleanup failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "err": err.Error()}})
		}
	}
	return UnregisterInstance(pid)
}

func CurrentPID() int {
	return os.Getpid()
}
