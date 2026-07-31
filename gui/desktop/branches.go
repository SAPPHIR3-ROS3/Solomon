package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type desktopProjectBranches struct {
	Current  string   `json:"current"`
	Branches []string `json:"branches"`
	IsRepo   bool     `json:"isRepo"`
}

type desktopProjectWorktree struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Bare    bool   `json:"bare"`
	Current bool   `json:"current"`
}

type desktopProjectWorktrees struct {
	Worktrees []desktopProjectWorktree `json:"worktrees"`
}

func (DesktopBridge) ProjectBranches(projectID string) (desktopProjectBranches, error) {
	return loadDesktopProjectBranches(projectID)
}

func (DesktopBridge) ProjectWorktrees(projectID string) (desktopProjectWorktrees, error) {
	return loadDesktopProjectWorktrees(projectID)
}

func (DesktopBridge) CheckoutProjectBranch(projectID, branch string) (desktopProjectBranches, error) {
	branch = strings.TrimSpace(branch)
	if !validDesktopGitBranchName(branch) {
		return desktopProjectBranches{}, fmt.Errorf("invalid branch name")
	}
	root, err := desktopRegisteredProjectRoot(projectID)
	if err != nil {
		return desktopProjectBranches{}, err
	}
	if !desktopIsGitRepo(root) {
		return desktopProjectBranches{}, fmt.Errorf("project is not a git repository")
	}
	current, branches, err := desktopListGitBranches(root)
	if err != nil {
		return desktopProjectBranches{}, err
	}
	found := false
	for _, name := range branches {
		if name == branch {
			found = true
			break
		}
	}
	if !found {
		return desktopProjectBranches{}, fmt.Errorf("branch not found")
	}
	if branch != current {
		if err := exec.Command("git", "-C", root, "checkout", branch).Run(); err != nil {
			return desktopProjectBranches{}, fmt.Errorf("checkout branch: %w", err)
		}
	}
	return loadDesktopProjectBranches(projectID)
}

func loadDesktopProjectBranches(projectID string) (desktopProjectBranches, error) {
	root, err := desktopRegisteredProjectRoot(projectID)
	if err != nil {
		return desktopProjectBranches{}, err
	}
	if !desktopIsGitRepo(root) {
		return desktopProjectBranches{IsRepo: false}, nil
	}
	current, branches, err := desktopListGitBranches(root)
	if err != nil {
		return desktopProjectBranches{}, err
	}
	return desktopProjectBranches{Current: current, Branches: branches, IsRepo: true}, nil
}

func loadDesktopProjectWorktrees(projectID string) (desktopProjectWorktrees, error) {
	root, err := desktopRegisteredProjectRoot(projectID)
	if err != nil {
		return desktopProjectWorktrees{}, err
	}
	if !desktopCanListWorktrees(root) {
		return desktopProjectWorktrees{Worktrees: []desktopProjectWorktree{}}, nil
	}
	worktrees, err := desktopListGitWorktrees(root)
	if err != nil {
		return desktopProjectWorktrees{}, err
	}
	return desktopProjectWorktrees{Worktrees: worktrees}, nil
}

func desktopRegisteredProjectRoot(projectID string) (string, error) {
	projectPath, err := desktopRegisteredProjectPath(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Clean(projectPath))
}

func desktopIsGitRepo(root string) bool {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func desktopCanListWorktrees(root string) bool {
	if desktopIsGitRepo(root) {
		return true
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--is-bare-repository").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func desktopListGitWorktrees(root string) ([]desktopProjectWorktree, error) {
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	currentRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		currentRoot = root
	}
	worktrees := make([]desktopProjectWorktree, 0)
	var current desktopProjectWorktree
	hasCurrent := false
	flush := func() {
		if !hasCurrent {
			return
		}
		if current.Path != "" {
			absPath, absErr := filepath.Abs(filepath.Clean(current.Path))
			if absErr == nil {
				current.Path = absPath
			}
			current.Current = filepath.Clean(current.Path) == filepath.Clean(currentRoot)
			worktrees = append(worktrees, current)
		}
		current = desktopProjectWorktree{}
		hasCurrent = false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = desktopProjectWorktree{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
			hasCurrent = true
		case line == "bare":
			current.Bare = true
			hasCurrent = true
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
			hasCurrent = true
		case line == "detached":
			current.Branch = ""
			hasCurrent = true
		}
	}
	flush()
	sort.Slice(worktrees, func(i, j int) bool {
		if worktrees[i].Bare != worktrees[j].Bare {
			return !worktrees[i].Bare
		}
		return worktrees[i].Path < worktrees[j].Path
	})
	return worktrees, nil
}

func desktopListGitBranches(root string) (string, []string, error) {
	currentOut, err := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	if err != nil {
		return "", nil, fmt.Errorf("read current branch: %w", err)
	}
	branchOut, err := exec.Command("git", "-C", root, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil {
		return "", nil, fmt.Errorf("list branches: %w", err)
	}
	seen := make(map[string]struct{})
	branches := make([]string, 0)
	for _, line := range strings.Split(string(branchOut), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		branches = append(branches, name)
	}
	sort.Slice(branches, func(i, j int) bool {
		if branches[i] == "main" {
			return true
		}
		if branches[j] == "main" {
			return false
		}
		return branches[i] < branches[j]
	})
	return strings.TrimSpace(string(currentOut)), branches, nil
}

func validDesktopGitBranchName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") {
		return false
	}
	if strings.ContainsAny(name, " ~^:?*[\\") {
		return false
	}
	for _, runeValue := range name {
		if unicode.IsControl(runeValue) || runeValue == 0x7f {
			return false
		}
	}
	return true
}
