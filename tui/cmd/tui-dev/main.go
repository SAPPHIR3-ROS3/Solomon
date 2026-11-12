package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/dev"
)

func main() {
	directory := flag.String("directory", ".", "Go frontend directory to build and run")
	watch := flag.String("watch", "", "Additional source directory to watch")
	flag.Parse()
	absolute, err := filepath.Abs(*directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	roots := []string{absolute}
	if *watch != "" {
		watchRoot, pathErr := filepath.Abs(*watch)
		if pathErr != nil {
			fmt.Fprintln(os.Stderr, pathErr)
			os.Exit(1)
		}
		roots = append(roots, watchRoot)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err = dev.Run(ctx, dev.Options{Directory: absolute, Arguments: flag.Args(), WatchRoots: roots}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
