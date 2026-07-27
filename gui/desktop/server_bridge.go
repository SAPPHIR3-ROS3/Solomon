package main

import serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"

// ServerBridge exposes the active local Solomon server URL to the desktop UI.
// Wails serves its WebView from wails.localhost, which is not the dev server.
type ServerBridge struct{}

func (ServerBridge) URL() string {
	state, err := serverruntime.LoadState()
	if err != nil {
		return ""
	}
	return state.LocalURL
}
