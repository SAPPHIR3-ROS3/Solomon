// hot_install coordinates Make's source installation with native client shutdown.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: hot_install <solomon-binary> <make> <bin-directory>")
		os.Exit(1)
	}
	target, err := filepath.Abs(os.Args[1])
	if err != nil {
		fail(err)
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	run := func(name string, args ...string) error {
		command := exec.CommandContext(ctx, name, args...)
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
		return command.Run()
	}
	err = lifecycle.HotInstall(ctx, target, filepath.Join(root, "gui"), func() error {
		if err := run(os.Args[2], "install", "BIN_DIR="+os.Args[3]); err != nil {
			return err
		}
		if runtime.GOOS == "linux" {
			return run("bash", "scripts/install-desktop.sh", os.Args[3])
		}
		return nil
	})
	if err != nil {
		fail(err)
	}
	fmt.Println("Solomon daemon and previously open native clients restarted.")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hot-install failed:", err)
	os.Exit(1)
}
