package replcomplete

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type SessionSource interface {
	ReplCompleteCfg() *config.Root
	ReplCompleteProjHex() string
	ReplCompleteProjRoot() string
	ReplCompleteShellFirst() bool
	ReplCompleteSnapshotSession() *chatstore.Session
}

func EnvFrom(src SessionSource) ReplCompleteEnv {
	if src == nil {
		return ReplCompleteEnv{}
	}
	return ReplCompleteEnv{
		Cfg:            src.ReplCompleteCfg(),
		ProjHex:        src.ReplCompleteProjHex(),
		ProjRoot:       src.ReplCompleteProjRoot(),
		ReplShellFirst: src.ReplCompleteShellFirst(),
		Session:        src.ReplCompleteSnapshotSession,
	}
}
