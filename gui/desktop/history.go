package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type desktopProjectGitCommit struct {
	Author     string   `json:"author"`
	AuthoredAt string   `json:"authoredAt"`
	Hash       string   `json:"hash"`
	Parents    []string `json:"parents"`
	Refs       []string `json:"refs"`
	ShortHash  string   `json:"shortHash"`
	Subject    string   `json:"subject"`
}

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
	commits := make([]desktopProjectGitCommit, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\x00")
		if len(fields) < 7 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		shortHash := strings.TrimSpace(fields[1])
		subject := strings.TrimSpace(fields[4])
		if hash == "" || shortHash == "" || subject == "" {
			continue
		}
		commits = append(commits, desktopProjectGitCommit{
			Author:     strings.TrimSpace(fields[2]),
			AuthoredAt: strings.TrimSpace(fields[3]),
			Hash:       hash,
			Parents:    strings.Fields(fields[6]),
			Refs:       splitDesktopGitRefs(fields[5]),
			ShortHash:  shortHash,
			Subject:    subject,
		})
	}
	return commits
}

func splitDesktopGitRefs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	refs := make([]string, 0, 2)
	for _, value := range strings.Split(value, ",") {
		if ref := strings.TrimSpace(value); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}
