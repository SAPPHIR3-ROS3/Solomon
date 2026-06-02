package replcomplete

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

type SessionSource interface {
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
		ProjHex:        src.ReplCompleteProjHex(),
		ProjRoot:       src.ReplCompleteProjRoot(),
		ReplShellFirst: src.ReplCompleteShellFirst(),
		Session:        src.ReplCompleteSnapshotSession,
	}
}
