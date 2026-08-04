//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/providerui"
)

func main() {
	var request providerui.ConnectRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result, err := providerui.Connect(context.Background(), request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]string{
		"model":    result.CurrentModel,
		"provider": result.CurrentProvider,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
