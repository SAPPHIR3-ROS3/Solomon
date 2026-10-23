// build_version chooses a baseline from origin's tags, with a local fallback.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

func remoteTags(output string) []string {
	var tags []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[1], "refs/tags/") {
			tags = append(tags, strings.TrimPrefix(fields[1], "refs/tags/"))
		}
	}
	return tags
}

func main() {
	exact := commandOutput(3*time.Second, "git", "describe", "--tags", "--exact-match", "--match", "v*")
	dirty := commandOutput(3*time.Second, "git", "status", "--porcelain") != ""
	// ls-remote reads origin without fetching or changing the checkout's tags.
	candidates := remoteTags(commandOutput(5*time.Second, "git", "-c", "credential.interactive=false", "ls-remote", "--tags", "--refs", "origin", "v*"))
	if updater.DevelopmentBuildVersion("", false, candidates...) == "dev" {
		fmt.Fprintln(os.Stderr, "Origin tags unavailable; using local tags.")
		candidates = strings.Fields(commandOutput(3*time.Second, "git", "tag", "--list", "v*"))
	}
	// A local exact tag must not override the baseline selected from origin.
	baseline := updater.DevelopmentBuildVersion("", true, candidates...)
	if exact != strings.TrimSuffix(baseline, "-dev") {
		exact = ""
	}
	fmt.Println(updater.DevelopmentBuildVersion(exact, dirty, candidates...))
}
