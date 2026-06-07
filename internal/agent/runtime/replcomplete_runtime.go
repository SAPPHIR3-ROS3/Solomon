package agentruntime

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func (r *Runtime) ReplCompleteCfg() *config.Root {
	if r == nil {
		return nil
	}
	return r.Cfg
}

func (r *Runtime) ReplCompleteProjHex() string { return r.ProjHex }

func (r *Runtime) ReplCompleteProjRoot() string { return r.ProjRoot }

func (r *Runtime) ReplCompleteShellFirst() bool { return r.ReplShellFirst }

func (r *Runtime) ReplCompleteSnapshotSession() *chatstore.Session {
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
