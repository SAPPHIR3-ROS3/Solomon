package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/desktopgit"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
)

// projectAPI owns the non-chat project surfaces that used to be implemented
// by the Vite middleware or by the Wails bridge. Keeping these operations at
// the daemon boundary makes browser and desktop clients address the same
// workspace and Git state.
type projectAPI struct{}

type apiProjectDirectoryEntry struct {
	IsDirectory bool   `json:"isDirectory"`
	Name        string `json:"name"`
	Path        string `json:"path"`
}

type apiProjectBranches struct {
	Branches []string `json:"branches"`
	Current  string   `json:"current"`
	IsRepo   bool     `json:"isRepo"`
}

type apiProjectWorktree struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Bare    bool   `json:"bare"`
	Current bool   `json:"current"`
}

type apiProjectWorktrees struct {
	Worktrees []apiProjectWorktree `json:"worktrees"`
}

type apiProjectGitHistory struct {
	Commits []desktopgit.Commit `json:"commits"`
	Current string              `json:"current"`
	IsRepo  bool                `json:"isRepo"`
}

type apiProjectGitStatus struct {
	Staged  map[string]string `json:"staged"`
	Changes map[string]string `json:"changes"`
	IsRepo  bool              `json:"isRepo"`
}

type apiProjectRemovalInfo struct {
	DataPath         string `json:"dataPath"`
	DataSizeBytes    int64  `json:"dataSizeBytes"`
	ProjectPath      string `json:"projectPath"`
	ProjectSizeBytes int64  `json:"projectSizeBytes"`
}

func newProjectAPI() *projectAPI { return &projectAPI{} }

func (*projectAPI) handlesProjectRoute(path string) bool {
	parts := splitProjectRoute(path)
	if len(parts) == 1 {
		return true
	}
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "disk", "removal-info", "files", "research", "history", "status", "branches", "checkout", "worktrees":
		return true
	default:
		return false
	}
}

func (a *projectAPI) handleProjectRoute(w http.ResponseWriter, r *http.Request) {
	parts := splitProjectRoute(r.URL.Path)
	if len(parts) == 0 {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	projectID, err := decodePathPart(parts[0])
	if err != nil || !safeProjectID(projectID) {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid project id"))
		return
	}
	if _, err := registeredProjectRoot(projectID); err != nil {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}

	if len(parts) == 2 && parts[1] == "files" && r.Method == http.MethodGet {
		a.handleProjectFiles(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "research" && r.Method == http.MethodGet {
		a.handleProjectResearch(w, projectID)
		return
	}
	if len(parts) == 4 && parts[1] == "research" && parts[3] == "report" && r.Method == http.MethodGet {
		researchID, decodeErr := decodePathPart(parts[2])
		if decodeErr != nil {
			writeAPIError(w, http.StatusBadRequest, decodeErr)
			return
		}
		a.handleProjectResearchReport(w, projectID, researchID)
		return
	}
	if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
		a.handleProjectHistory(w, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodGet {
		a.handleProjectStatus(w, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "branches" && r.Method == http.MethodGet {
		a.handleProjectBranches(w, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "worktrees" && r.Method == http.MethodGet {
		a.handleProjectWorktrees(w, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "checkout" && r.Method == http.MethodPost {
		a.handleProjectCheckout(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "removal-info" && r.Method == http.MethodGet {
		a.handleProjectRemovalInfo(w, projectID)
		return
	}
	if (len(parts) == 1 || (len(parts) == 2 && parts[1] == "disk")) && r.Method == http.MethodDelete {
		a.handleProjectRemoval(w, projectID, len(parts) == 2)
		return
	}
	writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
}

func (a *projectAPI) handleProjectFiles(w http.ResponseWriter, r *http.Request, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	entries, err := directoryEntries(root, r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (a *projectAPI) handleProjectResearch(w http.ResponseWriter, projectID string) {
	jobs, err := listProjectResearch(projectID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (a *projectAPI) handleProjectResearchReport(w http.ResponseWriter, projectID, researchID string) {
	jobs, err := listProjectResearch(projectID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	for _, job := range jobs {
		if job.ID != researchID {
			continue
		}
		path, pathErr := chatstore.ResearchHTMLPath(projectID, job.Slug)
		if pathErr != nil {
			writeAPIError(w, http.StatusInternalServerError, pathErr)
			return
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			writeAPIError(w, http.StatusNotFound, readErr)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
}

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

func (a *projectAPI) handleProjectRemovalInfo(w http.ResponseWriter, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	dataRoot, err := paths.ProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiProjectRemovalInfo{
		DataPath:         dataRoot,
		DataSizeBytes:    directorySize(dataRoot),
		ProjectPath:      root,
		ProjectSizeBytes: directorySize(root),
	})
}

func (a *projectAPI) handleProjectRemoval(w http.ResponseWriter, projectID string, removeData bool) {
	if err := removeRegisteredProject(projectID, removeData); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (a *projectAPI) handleHomeDirectoryEntries(w http.ResponseWriter, r *http.Request) {
	root, err := os.UserHomeDir()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	entries, err := directoryEntries(root, r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
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

func (a *projectAPI) handleUserName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		UserName string `json:"userName"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.UserName = strings.TrimSpace(request.UserName)
	if len(request.UserName) > 120 {
		writeAPIError(w, http.StatusBadRequest, errors.New("user name is too long"))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	cfg.UserName = request.UserName
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"userName": request.UserName})
}

func (a *projectAPI) handleReasoningEffort(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		ReasoningEffort string `json:"reasoningEffort"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	effort, err := config.ParseReasoningEffortToken(request.ReasoningEffort)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	cfg.ReasoningEffort = effort
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reasoningEffort": effort})
}

func registeredProjectRoot(projectID string) (string, error) {
	if !safeProjectID(projectID) {
		return "", errors.New("invalid project id")
	}
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return "", err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return "", err
	}
	for root, id := range projectMap {
		if strings.EqualFold(id, projectID) {
			return filepath.Abs(filepath.Clean(root))
		}
	}
	return "", os.ErrNotExist
}

func homeDirectoryRoot(relative string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return confinedPath(home, relative, "invalid home directory")
}

func directoryEntries(root, relative string) ([]apiProjectDirectoryEntry, error) {
	target, err := confinedPath(root, relative, "invalid directory")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	relativeTarget, _ := filepath.Rel(root, target)
	result := make([]apiProjectDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := entry.Name()
		if relativeTarget != "." {
			entryPath = filepath.Join(relativeTarget, entryPath)
		}
		result = append(result, apiProjectDirectoryEntry{
			IsDirectory: entry.IsDir(),
			Name:        entry.Name(),
			Path:        filepath.ToSlash(entryPath),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func confinedPath(root, relative, message string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(root, relative))
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relativeTarget) || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return "", errors.New(message)
	}
	return target, nil
}

func listProjectResearch(projectID string) ([]research.JobRecord, error) {
	if _, err := registeredProjectRoot(projectID); err != nil {
		return nil, err
	}
	slugs, err := chatstore.ListResearchJobFiles(projectID)
	if err != nil {
		return nil, err
	}
	jobs := make([]research.JobRecord, 0, len(slugs))
	for _, slug := range slugs {
		var job research.JobRecord
		if err := chatstore.ReadResearchJobFile(projectID, slug, &job); err == nil {
			jobs = append(jobs, job)
		}
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		left, right := jobs[i].FinishedAt, jobs[j].FinishedAt
		if left.IsZero() {
			left = jobs[i].StartedAt
		}
		if right.IsZero() {
			right = jobs[j].StartedAt
		}
		return left.After(right)
	})
	return jobs, nil
}

func runGit(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
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

func homeRelativePath(home, target string) string {
	relative, err := filepath.Rel(home, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return target
	}
	if relative == "." {
		return ""
	}
	return filepath.ToSlash(relative)
}

func directorySize(root string) int64 {
	var size int64
	_ = filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

func removeRegisteredProject(projectID string, removeData bool) error {
	if !safeProjectID(projectID) {
		return errors.New("invalid project id")
	}
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return err
	}
	registered := make([]string, 0)
	for root, id := range projectMap {
		if strings.EqualFold(id, projectID) {
			registered = append(registered, root)
			delete(projectMap, root)
		}
	}
	if len(registered) == 0 {
		return errors.New("project is not registered")
	}
	if removeData {
		for _, root := range registered {
			clean, err := filepath.Abs(filepath.Clean(root))
			if err != nil {
				return err
			}
			if clean == string(filepath.Separator) {
				return errors.New("refusing to remove filesystem root")
			}
			if home, homeErr := os.UserHomeDir(); homeErr == nil {
				cleanHome, _ := filepath.Abs(filepath.Clean(home))
				if clean == cleanHome {
					return errors.New("refusing to remove home directory")
				}
			}
			if err := os.RemoveAll(clean); err != nil {
				return fmt.Errorf("remove project directory: %w", err)
			}
		}
		projectRoot, err := paths.ProjectRoot(projectID)
		if err != nil {
			return err
		}
		projectsDir, err := paths.ProjectsDir()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(projectsDir, projectRoot)
		if err != nil || relative != projectID {
			return errors.New("invalid project data path")
		}
		if err := os.RemoveAll(projectRoot); err != nil {
			return fmt.Errorf("remove project data: %w", err)
		}
	}
	return project.SaveMap(mapPath, projectMap)
}
