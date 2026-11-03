package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

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

func (a *projectAPI) handleProjectFile(w http.ResponseWriter, r *http.Request, projectID string) {
	root, err := registeredProjectRoot(projectID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	target, err := safeWorkspacePath(root, r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if r.Method == http.MethodGet {
		file, openErr := os.Open(target)
		if openErr != nil {
			writeAPIError(w, http.StatusNotFound, openErr)
			return
		}
		defer file.Close()
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		if strings.EqualFold(filepath.Ext(target), ".pdf") {
			w.Header().Set("Content-Type", "application/pdf")
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		return
	}
	if strings.EqualFold(filepath.Ext(target), ".pdf") {
		writeAPIError(w, http.StatusUnsupportedMediaType, errors.New("PDF files cannot be saved as text"))
		return
	}
	defer r.Body.Close()
	content, readErr := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if readErr != nil {
		writeAPIError(w, http.StatusBadRequest, readErr)
		return
	}
	if writeErr := os.WriteFile(target, content, 0o644); writeErr != nil {
		writeAPIError(w, http.StatusInternalServerError, writeErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func safeWorkspacePath(root, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", errors.New("file path is required")
	}
	// Resolve the workspace itself before comparing it with resolved entries.
	// Temporary directories can use aliases such as /var on macOS or short
	// directory names on Windows.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(root, relativePath))
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid project file")
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	resolvedRelative, err := filepath.Rel(root, resolved)
	if err != nil || filepath.IsAbs(resolvedRelative) || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid project file")
	}
	return target, nil
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

func (a *projectAPI) handleFilesystemDirectoryEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		name := strings.TrimSpace(request.Name)
		if request.Path == "" || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") || !filepath.IsLocal(name) {
			writeAPIError(w, http.StatusBadRequest, errors.New("invalid folder name or parent path"))
			return
		}
		listing, err := filesystemDirectoryListing(request.Path)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		target := filepath.Join(filepath.FromSlash(listing.Path), name)
		if err := os.Mkdir(target, 0o755); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, apiProjectDirectoryEntry{IsDirectory: true, Name: name, Path: filepath.ToSlash(target)})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	listing, err := filesystemDirectoryListing(r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func filesystemDirectoryListing(requestPath string) (apiFilesystemDirectoryListing, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return apiFilesystemDirectoryListing{}, err
	}
	home, err = filepath.Abs(filepath.Clean(home))
	if err != nil {
		return apiFilesystemDirectoryListing{}, err
	}
	home = filepath.ToSlash(home)

	if requestPath == "" {
		if runtime.GOOS == "windows" {
			entries := make([]apiProjectDirectoryEntry, 0, 4)
			for drive := 'A'; drive <= 'Z'; drive++ {
				target := string(drive) + `:\`
				info, err := os.Stat(target)
				if err != nil || !info.IsDir() {
					continue
				}
				entries = append(entries, apiProjectDirectoryEntry{
					IsDirectory: true,
					Name:        string(drive) + ":",
					Path:        filepath.ToSlash(target),
				})
			}
			return apiFilesystemDirectoryListing{HomePath: home, Entries: entries}, nil
		}
		requestPath = string(filepath.Separator)
	}

	filesystemHome := filepath.FromSlash(home)
	if requestPath == "~" {
		requestPath = filesystemHome
	} else if strings.HasPrefix(requestPath, "~/") || strings.HasPrefix(requestPath, "~\\") {
		requestPath = filepath.Join(filesystemHome, requestPath[2:])
	}
	if !filepath.IsAbs(requestPath) {
		return apiFilesystemDirectoryListing{}, fmt.Errorf("directory path must be absolute")
	}
	target, err := filepath.Abs(filepath.Clean(requestPath))
	if err != nil {
		return apiFilesystemDirectoryListing{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return apiFilesystemDirectoryListing{}, err
	}
	if !info.IsDir() {
		return apiFilesystemDirectoryListing{}, fmt.Errorf("path is not a directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return apiFilesystemDirectoryListing{}, err
	}
	result := make([]apiProjectDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, apiProjectDirectoryEntry{
			IsDirectory: entry.IsDir(),
			Name:        entry.Name(),
			Path:        filepath.ToSlash(filepath.Join(target, entry.Name())),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return result[i].Name < result[j].Name
	})
	return apiFilesystemDirectoryListing{HomePath: home, Path: filepath.ToSlash(target), Entries: result}, nil
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
