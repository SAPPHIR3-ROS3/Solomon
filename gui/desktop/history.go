package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/desktopgit"
)

type desktopProjectGitCommit = desktopgit.Commit

type desktopProjectGitHistory struct {
	Commits []desktopProjectGitCommit `json:"commits"`
	Current string                    `json:"current"`
	IsRepo  bool                      `json:"isRepo"`
}

// ProjectGitHistory returns the latest commits for one registered project.
func (DesktopBridge) ProjectGitHistory(projectID string) (desktopProjectGitHistory, error) {
	return loadDesktopProjectGitHistory(projectID)
}

func loadDesktopProjectGitHistory(projectID string) (desktopProjectGitHistory, error) {
	root, err := desktopRegisteredProjectRoot(projectID)
	if err != nil {
		return desktopProjectGitHistory{}, err
	}
	if !desktopIsGitRepo(root) {
		return desktopProjectGitHistory{Commits: []desktopProjectGitCommit{}, IsRepo: false}, nil
	}

	currentOut, err := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	if err != nil {
		return desktopProjectGitHistory{}, fmt.Errorf("read current branch: %w", err)
	}
	historyOut, err := exec.Command(
		"git", "-C", root, "log",
		"--no-color",
		"--decorate=short",
		"--date=iso-strict",
		"--topo-order",
		"--format=%H%x00%h%x00%an%x00%aI%x00%s%x00%D%x00%P",
	).Output()
	if err != nil {
		return desktopProjectGitHistory{}, fmt.Errorf("read Git history: %w", err)
	}

	return desktopProjectGitHistory{
		Commits: parseDesktopGitHistory(historyOut),
		Current: strings.TrimSpace(string(currentOut)),
		IsRepo:  true,
	}, nil
}

func parseDesktopGitHistory(output []byte) []desktopProjectGitCommit {
	return desktopgit.ParseHistory(output)
}
