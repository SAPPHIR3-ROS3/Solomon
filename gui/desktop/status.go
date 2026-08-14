package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type desktopProjectGitStatus struct {
	Staged  map[string]string `json:"staged"`
	Changes map[string]string `json:"changes"`
	IsRepo  bool              `json:"isRepo"`
}

// ProjectGitStatus returns the staged and unstaged files for one registered project.
func (DesktopBridge) ProjectGitStatus(projectID string) (desktopProjectGitStatus, error) {
	return loadDesktopProjectGitStatus(projectID)
}

func loadDesktopProjectGitStatus(projectID string) (desktopProjectGitStatus, error) {
	root, err := desktopRegisteredProjectRoot(projectID)
	if err != nil {
		return desktopProjectGitStatus{}, err
	}
	if !desktopIsGitRepo(root) {
		return desktopProjectGitStatus{
			Changes: map[string]string{},
			Staged:  map[string]string{},
			IsRepo:  false,
		}, nil
	}

	stagedOutput, err := desktopGitStatusCommand(root, "diff", "--cached", "--name-status", "-z")
	if err != nil {
		return desktopProjectGitStatus{}, fmt.Errorf("read staged Git files: %w", err)
	}
	changesOutput, err := desktopGitStatusCommand(root, "diff", "--name-status", "-z")
	if err != nil {
		return desktopProjectGitStatus{}, fmt.Errorf("read changed Git files: %w", err)
	}
	untrackedOutput, err := desktopGitStatusCommand(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return desktopProjectGitStatus{}, fmt.Errorf("read untracked Git files: %w", err)
	}

	changes := parseDesktopGitStatus(changesOutput)
	for _, filePath := range strings.Split(string(untrackedOutput), "\x00") {
		if normalizedPath := normalizeDesktopGitPath(filePath); normalizedPath != "" {
			changes[normalizedPath] = "U"
		}
	}
	return desktopProjectGitStatus{
		Changes: changes,
		Staged:  parseDesktopGitStatus(stagedOutput),
		IsRepo:  true,
	}, nil
}

func desktopGitStatusCommand(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}

func parseDesktopGitStatus(output []byte) map[string]string {
	status := make(map[string]string)
	fields := strings.Split(string(output), "\x00")
	for index := 0; index < len(fields); {
		code := strings.TrimSpace(fields[index])
		index++
		if code == "" {
			continue
		}
		statusCode := code[0]
		if statusCode == 'R' || statusCode == 'C' {
			if index < len(fields) {
				index++
			}
			if index < len(fields) {
				if filePath := normalizeDesktopGitPath(fields[index]); filePath != "" {
					status[filePath] = string(statusCode)
				}
				index++
			}
			continue
		}
		if index >= len(fields) {
			break
		}
		if filePath := normalizeDesktopGitPath(fields[index]); filePath != "" {
			status[filePath] = string(statusCode)
		}
		index++
	}
	return status
}

func normalizeDesktopGitPath(value string) string {
	return strings.TrimSuffix(value, "\r")
}
