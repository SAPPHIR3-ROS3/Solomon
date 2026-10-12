// build_version chooses a development baseline without regressing an installed release.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func commandOutput(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 || (len(os.Args) == 3 && os.Args[2] != "--latest") {
		fmt.Fprintln(os.Stderr, "usage: build_version <installed-cli> [--latest]")
		os.Exit(2)
	}
	exact := commandOutput(3*time.Second, "git", "describe", "--tags", "--exact-match", "--match", "v*")
	dirty := commandOutput(3*time.Second, "git", "status", "--porcelain") != ""
	candidates := strings.Fields(commandOutput(3*time.Second, "git", "tag", "--list", "v*"))
	// --binary skips daemon and desktop setup: version resolution must never
	// start a service or install an application while Make parses its targets.
	installed := commandOutput(3*time.Second, os.Args[1], "version", "--binary")
	for _, line := range strings.Split(installed, "\n") {
		candidates = append(candidates, line)
	}
	if len(os.Args) == 3 {
		logging.LogInit(logging.ERROR_LOG_LEVEL)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result := updater.Check(ctx, "dev")
		cancel()
		if result.Err == nil {
			candidates = append(candidates, result.LatestTag)
		} else {
			fmt.Fprintln(os.Stderr, "Latest release unavailable; using local tags and installed version.")
		}
	}
	fmt.Println(updater.DevelopmentBuildVersion(exact, dirty, candidates...))
}
