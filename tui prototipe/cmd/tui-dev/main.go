package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/dev"
)

func main() {
	directory, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	options := dev.Options{Directory: directory, Arguments: os.Args[1:], WatchRoots: []string{directory, filepath.Join(directory, "..", "tui")}}
	if err = dev.Run(ctx, options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
