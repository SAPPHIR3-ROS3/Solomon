package turnloop

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func saveMainMessageChange(h Host, operation string, fn func(*chatstore.Session) int) {
	if host, ok := h.(interface {
		SaveMainMessageChange(func(*chatstore.Session) int) error
	}); ok {
		if err := host.SaveMainMessageChange(fn); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"context": operation, "err": err.Error()}})
		}
		return
	}
	h.MutateSession(func(s *chatstore.Session) { fn(s) })
	h.PersistSessionOrLog(operation)
}

func mutateSessionMetadata(h Host, fn func(*chatstore.Session)) {
	if host, ok := h.(interface {
		MutateSessionMetadata(func(*chatstore.Session))
	}); ok {
		host.MutateSessionMetadata(fn)
		return
	}
	h.MutateSession(fn)
}
