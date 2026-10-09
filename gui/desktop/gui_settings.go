//go:build ignore

// Development Vite servers use the same config persistence as the daemon.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func main() {
	var request struct {
		Action   string                 `json:"action"`
		Chat     config.GUIChatSettings `json:"chat"`
		Provider string                 `json:"provider"`
		Model    string                 `json:"model"`
		Enabled  *bool                  `json:"enabled"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(err)
	}
	var settings config.GUISettings
	var err error
	switch request.Action {
	case "read":
		settings, err = config.ReadGUISettings()
	case "patch":
		settings, err = config.UpdateGUIChatSettings(request.Chat)
	case "visibility":
		if request.Enabled == nil {
			fail(fmt.Errorf("enabled is required"))
		}
		err = config.UpdateModelVisibility(request.Provider, request.Model, *request.Enabled)
		if err == nil {
			settings, err = config.ReadGUISettings()
		}
	default:
		fail(fmt.Errorf("unknown GUI settings action"))
	}
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(settings); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
