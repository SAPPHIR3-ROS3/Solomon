package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (DesktopBridge) RemoveProjectFromSidebar(projectID string) error {
	return removeDesktopProject(projectID, false)
}

// RemoveProjectFromDisk unregisters a project and permanently deletes its
// Solomon project data, including chats.
func (DesktopBridge) RemoveProjectFromDisk(projectID string) error {
	return removeDesktopProject(projectID, true)
}

func (DesktopBridge) ProjectRemovalInfo(projectID string) (desktopProjectRemovalInfo, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectPath := ""
	for registeredPath, registeredID := range projectMap {
		if registeredID == projectID {
			projectPath = registeredPath
			break
		}
	}
	if projectPath == "" {
		return desktopProjectRemovalInfo{}, fmt.Errorf("project is not registered")
	}
	absoluteProjectPath, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	dataPath, err := paths.ProjectRoot(projectID)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectSize, err := desktopDirectorySize(absoluteProjectPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	dataSize, err := desktopDirectorySize(dataPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	return desktopProjectRemovalInfo{DataPath: dataPath, DataSizeBytes: dataSize, ProjectPath: absoluteProjectPath, ProjectSizeBytes: projectSize}, nil
}

// ProjectDirectoryEntries returns one directory level from a registered project.
// The relative path is constrained to the project root before it is read.
func (DesktopBridge) ProjectDirectoryEntries(projectID, relativePath string) ([]desktopProjectDirectoryEntry, error) {
	projectPath, err := desktopRegisteredProjectPath(projectID)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return nil, err
	}
	target := filepath.Clean(filepath.Join(root, relativePath))
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relativeTarget) || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid project directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	result := make([]desktopProjectDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := entry.Name()
		if relativeTarget != "." {
			entryPath = filepath.Join(relativeTarget, entryPath)
		}
		result = append(result, desktopProjectDirectoryEntry{Name: entry.Name(), Path: entryPath, IsDirectory: entry.IsDir()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// HomeDirectoryEntries returns one directory level below the user's home
// directory. The requested path is constrained to that home directory.
func (DesktopBridge) HomeDirectoryEntries(relativePath string) ([]desktopProjectDirectoryEntry, error) {
	root, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	target := filepath.Clean(filepath.Join(root, relativePath))
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relativeTarget) || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid home directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	result := make([]desktopProjectDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := entry.Name()
		if relativeTarget != "." {
			entryPath = filepath.Join(relativeTarget, entryPath)
		}
		result = append(result, desktopProjectDirectoryEntry{IsDirectory: entry.IsDir(), Name: entry.Name(), Path: entryPath})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// ProjectAtMentionSuggestions delegates matching and tag shortening to the
// same atmention package used by Solomon's terminal editor.
func (DesktopBridge) ProjectAtMentionSuggestions(projectID, query string) ([]desktopAtMentionSuggestion, error) {
	projectPath, err := desktopRegisteredProjectPath(projectID)
	if err != nil {
		return nil, err
	}
	entries, err := desktopAtMentionIndex.Get(context.Background(), projectPath)
	if err != nil {
		return nil, err
	}
	return desktopAtMentionMatches(entries, query), nil
}

// AtMentionSuggestions lets virtual GUI folders (such as test chats) use the
// terminal's matching and tag-shortening implementation without duplicating it
// in TypeScript.
func (DesktopBridge) AtMentionSuggestions(entries []desktopAtMentionEntry, query string) []desktopAtMentionSuggestion {
	atEntries := make([]atmention.Entry, 0, len(entries))
	for _, entry := range entries {
		atEntries = append(atEntries, atmention.Entry{IsDir: entry.IsDirectory, RelPath: entry.Path})
	}
	return desktopAtMentionMatches(atEntries, query)
}

func desktopAtMentionMatches(entries []atmention.Entry, query string) []desktopAtMentionSuggestion {
	matches := atmention.MatchQuery(query, entries, atmention.MaxPickerResults)
	if strings.TrimSpace(query) == "" {
		matches = atmention.InitialPickerEntries(entries, atmention.MaxPickerResults)
	}
	result := make([]desktopAtMentionSuggestion, 0, len(matches))
	for _, entry := range matches {
		result = append(result, desktopAtMentionSuggestion{
			IsDirectory: entry.IsDir,
			Path:        entry.RelPath,
			Tag:         "@" + atmention.ShortTag(entry.RelPath, entries),
		})
	}
	return result
}

// ProjectResearch returns deep-research jobs persisted for one registered project.
func (DesktopBridge) ProjectResearch(projectID string) ([]research.JobRecord, error) {
	if _, err := desktopRegisteredProjectPath(projectID); err != nil {
		return nil, err
	}
	slugs, err := chatstore.ListResearchJobFiles(projectID)
	if err != nil {
		return []research.JobRecord{}, nil
	}
	jobs := make([]research.JobRecord, 0, len(slugs))
	for _, slug := range slugs {
		var job research.JobRecord
		if err := chatstore.ReadResearchJobFile(projectID, slug, &job); err == nil {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
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

func (DesktopBridge) ProjectResearchReport(projectID, researchID string) (string, error) {
	jobs, err := (DesktopBridge{}).ProjectResearch(projectID)
	if err != nil {
		return "", err
	}
	for _, job := range jobs {
		if job.ID != researchID {
			continue
		}
		path, err := chatstore.ResearchHTMLPath(projectID, job.Slug)
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
	return "", fmt.Errorf("research report not found")
}

func desktopRegisteredProjectPath(projectID string) (string, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return "", err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return "", err
	}
	for projectPath, registeredID := range projectMap {
		if registeredID == projectID {
			return projectPath, nil
		}
	}
	return "", fmt.Errorf("project is not registered")
}

func desktopDirectorySize(directory string) (int64, error) {
	var size int64
	err := filepath.WalkDir(directory, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		size += info.Size()
		return nil
	})
	if os.IsNotExist(err) || err != nil {
		return size, nil
	}
	return size, nil
}

func removeDesktopProject(projectID string, removeData bool) error {
	if len(projectID) != 64 {
		return fmt.Errorf("invalid project id")
	}
	for _, char := range projectID {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return fmt.Errorf("invalid project id")
		}
	}

	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return err
	}
	found := false
	registeredPaths := make([]string, 0, 1)
	for projectPath, registeredID := range projectMap {
		if registeredID == projectID {
			found = true
			registeredPaths = append(registeredPaths, projectPath)
		}
	}
	if !found {
		return fmt.Errorf("project is not registered")
	}

	if removeData {
		for _, registeredPath := range registeredPaths {
			if err := removeDesktopProjectDirectory(registeredPath); err != nil {
				return err
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
		relativeRoot, err := filepath.Rel(projectsDir, projectRoot)
		if err != nil || relativeRoot != projectID {
			return fmt.Errorf("invalid project data path")
		}
		if err := os.RemoveAll(projectRoot); err != nil {
			return fmt.Errorf("remove project data: %w", err)
		}
	}
	return project.UnregisterProject(mapPath, projectID, removeData)
}

func removeDesktopProjectDirectory(projectPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}
	if cleanPath == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove filesystem root")
	}
	if home, err := os.UserHomeDir(); err == nil {
		cleanHome, _ := filepath.Abs(filepath.Clean(home))
		if cleanPath == cleanHome {
			return fmt.Errorf("refusing to remove home directory")
		}
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove project directory: %w", err)
	}
	return nil
}

func loadDesktopProjects() ([]desktopProject, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(mapPath)
	if os.IsNotExist(err) {
		return []desktopProject{}, nil
	}
	if err != nil {
		return nil, err
	}
	var projectIDs map[string]string
	if err := json.Unmarshal(data, &projectIDs); err != nil {
		return nil, fmt.Errorf("read projectsId.json: %w", err)
	}

	projects := make([]desktopProject, 0, len(projectIDs))
	for projectPath, projectID := range projectIDs {
		if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(projectID) == "" {
			continue
		}
		chats, err := chatstore.ListRecent(projectID, 10_000)
		if err != nil {
			return nil, err
		}
		_, userTokens, reasoningTokens, responseTokens, statsErr := chatstore.ProjectWelcomeStats(projectID)
		tokenStats := desktopTokenStats{}
		if statsErr == nil {
			tokenStats = desktopTokenStats{
				User: userTokens, Reasoning: reasoningTokens, Response: responseTokens,
				Total: userTokens + reasoningTokens + responseTokens,
			}
		}
		project := desktopProject{ID: projectID, Name: desktopProjectDisplayName(projectPath), Path: projectPath, Chats: make([]desktopChat, 0, len(chats)), ChatCount: len(chats), TokenStats: tokenStats}
		if project.Name == "." || project.Name == string(filepath.Separator) {
			project.Name = projectPath
		}
		for _, chat := range chats {
			activity := chatstore.SessionActivityTime(chat)
			project.Chats = append(project.Chats, desktopChat{ID: chat.ID, LastMessageAt: activity.UTC().Format(time.RFC3339), Title: chat.Title})
			if activity.After(project.activity) {
				project.activity = activity
			}
		}
		if project.activity.IsZero() {
			if projectRoot, err := paths.ProjectRoot(projectID); err == nil {
				if info, err := os.Stat(projectRoot); err == nil {
					project.activity = info.ModTime()
				}
			}
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].activity.Equal(projects[j].activity) {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].activity.After(projects[j].activity)
	})
	return projects, nil
}

func desktopProjectDisplayName(projectPath string) string {
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(projectPath) == filepath.Clean(home) {
		return "Home"
	}
	name := filepath.Base(projectPath)
	if name == "." || name == string(filepath.Separator) {
		return projectPath
	}
	return name
}
