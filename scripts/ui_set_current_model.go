//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ui_set_current_model.go <provider> <model>")
		os.Exit(1)
	}
	providerName := strings.TrimSpace(os.Args[1])
	modelID := strings.TrimSpace(os.Args[2])
	if providerName == "" || modelID == "" {
		fmt.Fprintln(os.Stderr, "provider and model are required")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, ok := cfg.Providers[providerName]; !ok {
		fmt.Fprintf(os.Stderr, "unknown provider %q\n", providerName)
		os.Exit(1)
	}
	changed := cfg.Current.Provider != providerName || cfg.Current.Model != modelID
	cfg.Current.Provider = providerName
	cfg.Current.Model = modelID
	if changed {
		config.NoteRecentModelUse(cfg, providerName, modelID)
	}
	if err := config.Save(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
		"provider": providerName,
		"model":    modelID,
	})
}
