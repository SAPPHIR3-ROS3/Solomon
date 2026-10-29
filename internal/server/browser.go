package server

import (
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func (a *chatAPI) attachBrowser(runtime *agentruntime.Runtime, projectID, chatID string) {
	if a.browsers == nil {
		return
	}
	client, err := a.browsers.Get(projectID, chatID)
	if err != nil {
		logging.Log(logging.INFO_LOG_LEVEL, "chat browser unavailable", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return
	}
	runtime.Cloak = client
	runtime.CloakShared = true
}
