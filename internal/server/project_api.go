package server

import (
	"errors"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/desktopgit"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type apiFilesystemDirectoryListing struct {
	HomePath string                     `json:"homePath"`
	Path     string                     `json:"path"`
	Entries  []apiProjectDirectoryEntry `json:"entries"`
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
	case "disk", "removal-info", "file-operation", "file", "files", "research", "history", "status", "stage", "unstage", "restore", "commit", "branches", "checkout", "worktrees":
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

	if len(parts) == 2 && parts[1] == "file-operation" && r.Method == http.MethodPost {
		a.handleProjectFileOperation(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "files" && r.Method == http.MethodGet {
		a.handleProjectFiles(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "file" && (r.Method == http.MethodGet || r.Method == http.MethodPut) {
		a.handleProjectFile(w, r, projectID)
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
	if len(parts) == 2 && parts[1] == "commit" && r.Method == http.MethodPost {
		a.handleProjectCommit(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "stage" && r.Method == http.MethodPost {
		a.handleProjectStage(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "restore" && r.Method == http.MethodPost {
		a.handleProjectRestore(w, r, projectID)
		return
	}
	if len(parts) == 2 && parts[1] == "unstage" && r.Method == http.MethodPost {
		a.handleProjectUnstage(w, r, projectID)
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
