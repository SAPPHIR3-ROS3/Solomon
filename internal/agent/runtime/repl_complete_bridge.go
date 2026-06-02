package agentruntime

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func (r *Runtime) replCompleteEnv() replcomplete.ReplCompleteEnv {
	return replcomplete.ReplCompleteEnv{
		ProjHex:        r.ProjHex,
		ProjRoot:       r.ProjRoot,
		ReplShellFirst: r.ReplShellFirst,
		Session:        r.snapshotSession,
	}
}

func (r *Runtime) snapshotSession() *chatstore.Session {
	var snap *chatstore.Session
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		cp := *s
		snap = &cp
	})
	return snap
}
