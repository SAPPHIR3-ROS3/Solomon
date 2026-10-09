// install_local commits prebuilt Windows artifacts using the common handoff.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: install_local <staged-cli> <built-desktop> <target-cli> <version>")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	err := lifecycle.HotInstall(ctx, os.Args[3], "", func() error {
		if err := updater.CommitInstall(os.Args[1], os.Args[3]); err != nil {
			return err
		}
		return updater.InstallBuiltDesktop(ctx, os.Args[4], os.Args[3], os.Args[2], os.Stdout)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
