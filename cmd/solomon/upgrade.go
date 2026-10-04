package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

const upgradeSmokeMarker = "solomon-cli-upgrade-v1"

func runUpgradeCLI() {
	ctx := context.Background()
	logging.LogInit(logging.INFO_LOG_LEVEL)
	logging.Log(logging.INFO_LOG_LEVEL, "upgrade cli", logging.LogOptions{Params: map[string]any{"marker": upgradeSmokeMarker}})
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	state, err := serverruntime.EnsureRunning(ctx, executable)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	health, err := serverruntime.ReadHealth(ctx, state)
	if err != nil || !health.OK {
		fmt.Fprintln(os.Stderr, "Solomon daemon is unavailable:", err)
		os.Exit(1)
	}
	current, commit, commitTime, sourceTree := health.Server.Version, health.Server.Commit, health.Server.CommitTime, health.Server.SourceTree
	res := updater.CheckWithSourceTree(ctx, current, commit, commitTime, sourceTree)
	if res.Err != nil {
		fmt.Fprintln(os.Stderr, res.Err)
		os.Exit(1)
	}
	if res.LocalCommitRelation == "ahead" || res.LocalCommitRelation == "identical" {
		commands.SetEffectiveReleaseVersion(res.LatestTag, res.MatchesReleaseSource)
		current = commands.VersionString()
	}
	if !res.Newer {
		fmt.Fprintf(os.Stdout, "Solomon is up to date (%s)\n", current)
		os.Exit(0)
	}
	notice := res.Notice()
	fmt.Fprintf(os.Stdout, "Installing %s...\n", notice.Latest)
	err = updater.RunSystemInstall(ctx, notice.Latest, os.Stdout)
	if errors.Is(err, updater.ErrRestartScheduled) {
		if finishErr := updater.FinishUpgradeRestart(ctx, notice.Latest); finishErr != nil {
			fmt.Fprintln(os.Stderr, finishErr)
			fmt.Fprintln(os.Stderr, updater.InstallFallbackMessage(notice.Latest))
			os.Exit(1)
		}
		if runtime.GOOS == "windows" {
			os.Exit(0)
		}
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, updater.InstallFallbackMessage(notice.Latest))
		os.Exit(1)
	}
}
