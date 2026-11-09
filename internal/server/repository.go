package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/desktopgit"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (a *projectAPI) handleProjectHistory(w http.ResponseWriter, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	history, err := gitHistory(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (a *projectAPI) handleProjectStatus(w http.ResponseWriter, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	status, err := gitStatus(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *projectAPI) handleProjectStage(w http.ResponseWriter, r *http.Request, projectID string) {
	var request struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSONBody(w, r, &request, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	status, err := stageGitPaths(root, request.Paths)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *projectAPI) handleProjectUnstage(w http.ResponseWriter, r *http.Request, projectID string) {
	var request struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSONBody(w, r, &request, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	status, err := unstageGitPaths(root, request.Paths)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *projectAPI) handleProjectRestore(w http.ResponseWriter, r *http.Request, projectID string) {
	var request struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSONBody(w, r, &request, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	status, err := restoreGitPaths(root, request.Paths)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *projectAPI) handleProjectBranches(w http.ResponseWriter, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	branches, err := gitBranches(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (a *projectAPI) handleProjectWorktrees(w http.ResponseWriter, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	worktrees, err := gitWorktrees(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, worktrees)
}

func (a *projectAPI) handleProjectCheckout(w http.ResponseWriter, r *http.Request, projectID string) {
	var request struct {
		Branch string `json:"branch"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	branches, err := checkoutGitBranch(root, request.Branch)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (a *projectAPI) handleHomeBranches(w http.ResponseWriter, r *http.Request) {
	root, err := homeDirectoryRoot(r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	branches, err := gitBranches(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (a *projectAPI) handleHomeWorktrees(w http.ResponseWriter, r *http.Request) {
	root, err := homeDirectoryRoot(r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	worktrees, err := gitWorktrees(root)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	home, _ := os.UserHomeDir()
	for index := range worktrees.Worktrees {
		worktrees.Worktrees[index].Path = homeRelativePath(home, worktrees.Worktrees[index].Path)
	}
	writeJSON(w, http.StatusOK, worktrees)
}

func (a *projectAPI) handleHomeCheckout(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Branch string `json:"branch"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := homeDirectoryRoot(r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	branches, err := checkoutGitBranch(root, request.Branch)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func runGit(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	configureManagedProcess(command)
	started := time.Now()
	output, err := command.Output()
	logging.ProcessFinished(ctx, command, "repository."+args[0], started, err)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func isGitWorkTree(root string) bool {
	value, err := runGit(root, "rev-parse", "--is-inside-work-tree")
	return err == nil && value == "true"
}

func isGitRootOrBare(root string) bool {
	if isGitWorkTree(root) {
		return true
	}
	value, err := runGit(root, "rev-parse", "--is-bare-repository")
	return err == nil && value == "true"
}

func gitBranches(root string) (apiProjectBranches, error) {
	if !isGitWorkTree(root) {
		return apiProjectBranches{Branches: []string{}, IsRepo: false}, nil
	}
	current, err := runGit(root, "branch", "--show-current")
	if err != nil {
		return apiProjectBranches{}, fmt.Errorf("read current branch: %w", err)
	}
	branchOutput, err := runGit(root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return apiProjectBranches{}, fmt.Errorf("list branches: %w", err)
	}
	seen := map[string]bool{}
	branches := make([]string, 0)
	for _, value := range strings.Split(branchOutput, "\n") {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			branches = append(branches, value)
		}
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
	return apiProjectBranches{Branches: branches, Current: current, IsRepo: true}, nil
}

func gitHistory(root string) (apiProjectGitHistory, error) {
	if !isGitWorkTree(root) {
		return apiProjectGitHistory{Commits: []desktopgit.Commit{}, IsRepo: false}, nil
	}
	current, err := runGit(root, "branch", "--show-current")
	if err != nil {
		return apiProjectGitHistory{}, fmt.Errorf("read current branch: %w", err)
	}
	output, err := runGit(root, "log", "--no-color", "--decorate=short", "--date=iso-strict", "--topo-order", "--format=%H%x00%h%x00%an%x00%aI%x00%s%x00%D%x00%P")
	if err != nil {
		return apiProjectGitHistory{}, fmt.Errorf("read Git history: %w", err)
	}
	return apiProjectGitHistory{Commits: desktopgit.ParseHistory([]byte(output)), Current: current, IsRepo: true}, nil
}

func gitStatus(root string) (apiProjectGitStatus, error) {
	if !isGitWorkTree(root) {
		return apiProjectGitStatus{Changes: map[string]string{}, Staged: map[string]string{}, IsRepo: false}, nil
	}
	staged, err := runGit(root, "diff", "--cached", "--name-status", "-z")
	if err != nil {
		return apiProjectGitStatus{}, fmt.Errorf("read staged Git files: %w", err)
	}
	changes, err := runGit(root, "diff", "--name-status", "-z")
	if err != nil {
		return apiProjectGitStatus{}, fmt.Errorf("read changed Git files: %w", err)
	}
	untracked, err := runGit(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return apiProjectGitStatus{}, fmt.Errorf("read untracked Git files: %w", err)
	}
	changeMap := desktopgit.ParseStatus([]byte(changes))
	for _, path := range strings.Split(untracked, "\x00") {
		path = strings.TrimSuffix(path, "\r")
		if path != "" {
			changeMap[path] = "U"
		}
	}
	return apiProjectGitStatus{Changes: changeMap, IsRepo: true, Staged: desktopgit.ParseStatus([]byte(staged))}, nil
}

func stageGitPaths(root string, paths []string) (apiProjectGitStatus, error) {
	return applyGitIndex(root, []string{"add", "--"}, paths, "stage git paths")
}

func restoreGitPaths(root string, paths []string) (apiProjectGitStatus, error) {
	return applyGitIndex(root, []string{"--literal-pathspecs", "restore", "--worktree", "--"}, paths, "restore git paths")
}

func unstageGitPaths(root string, paths []string) (apiProjectGitStatus, error) {
	return applyGitIndex(root, []string{"restore", "--staged", "--"}, paths, "unstage git paths")
}

func applyGitIndex(root string, command, paths []string, action string) (apiProjectGitStatus, error) {
	if !isGitWorkTree(root) {
		return apiProjectGitStatus{}, fmt.Errorf("not a git repository")
	}
	args := append([]string{}, command...)
	for _, path := range paths {
		path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
		path = strings.TrimPrefix(path, "./")
		if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
			return apiProjectGitStatus{}, fmt.Errorf("invalid git path")
		}
		args = append(args, path)
	}
	if len(args) == len(command) {
		return apiProjectGitStatus{}, fmt.Errorf("no paths to update")
	}
	if _, err := runGit(root, args...); err != nil {
		return apiProjectGitStatus{}, fmt.Errorf("%s: %w", action, err)
	}
	return gitStatus(root)
}

func validGitBranchName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") || strings.ContainsAny(name, " ~^:?*[\\") {
		return false
	}
	for _, value := range name {
		if value < 0x20 || value == 0x7f {
			return false
		}
	}
	return true
}

func checkoutGitBranch(root, branch string) (apiProjectBranches, error) {
	branch = strings.TrimSpace(branch)
	if !validGitBranchName(branch) {
		return apiProjectBranches{}, errors.New("invalid branch name")
	}
	info, err := gitBranches(root)
	if err != nil {
		return apiProjectBranches{}, err
	}
	if !info.IsRepo {
		return apiProjectBranches{}, errors.New("project is not a git repository")
	}
	found := false
	for _, value := range info.Branches {
		if value == branch {
			found = true
			break
		}
	}
	if !found {
		return apiProjectBranches{}, errors.New("branch not found")
	}
	if branch != info.Current {
		if _, err := runGit(root, "checkout", branch); err != nil {
			return apiProjectBranches{}, fmt.Errorf("checkout branch: %w", err)
		}
	}
	return gitBranches(root)
}

func gitWorktrees(root string) (apiProjectWorktrees, error) {
	if !isGitRootOrBare(root) {
		return apiProjectWorktrees{Worktrees: []apiProjectWorktree{}}, nil
	}
	output, err := runGit(root, "worktree", "list", "--porcelain")
	if err != nil {
		return apiProjectWorktrees{}, fmt.Errorf("list worktrees: %w", err)
	}
	currentRoot, _ := filepath.Abs(filepath.Clean(root))
	result := make([]apiProjectWorktree, 0)
	var current apiProjectWorktree
	hasCurrent := false
	flush := func() {
		if !hasCurrent || current.Path == "" {
			current = apiProjectWorktree{}
			hasCurrent = false
			return
		}
		current.Path, _ = filepath.Abs(filepath.Clean(current.Path))
		current.Current = filepath.Clean(current.Path) == filepath.Clean(currentRoot)
		result = append(result, current)
		current = apiProjectWorktree{}
		hasCurrent = false
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			hasCurrent = true
		case line == "bare":
			current.Bare = true
			hasCurrent = true
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "branch ")), "refs/heads/")
			hasCurrent = true
		case line == "detached":
			current.Branch = ""
			hasCurrent = true
		}
	}
	flush()
	sort.Slice(result, func(i, j int) bool {
		if result[i].Bare != result[j].Bare {
			return !result[i].Bare
		}
		return result[i].Path < result[j].Path
	})
	return apiProjectWorktrees{Worktrees: result}, nil
}
