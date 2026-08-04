//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type modelVisibilityRequest struct {
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type modelVisibilityResponse struct {
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

func main() {
	var request modelVisibilityRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := config.UpdateModelVisibility(request.Provider, request.Model, request.Enabled); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(modelVisibilityResponse{
		Enabled:  request.Enabled,
		Model:    request.Model,
		Provider: request.Provider,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
