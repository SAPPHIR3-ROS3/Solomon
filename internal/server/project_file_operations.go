package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type projectFileOperation struct {
	Action      string `json:"action"`
	Path        string `json:"path"`
	Destination string `json:"destination"`
}

func (a *projectAPI) handleProjectFileOperation(w http.ResponseWriter, r *http.Request, projectID string) {
	var request projectFileOperation
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	root, err := registeredProjectRoot(projectID)
	if err == nil {
		err = performProjectFileOperation(root, request)
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func performProjectFileOperation(root string, request projectFileOperation) error {
	if request.Action != "rename" && request.Action != "delete" && request.Action != "copy" && request.Action != "move" {
		return errors.New("invalid file action")
	}
	source, err := safeWorkspacePath(root, request.Path)
	if err != nil {
		return err
	}
	if source == filepath.Clean(root) {
		return errors.New("cannot modify workspace root")
	}
	if request.Action == "delete" {
		return os.RemoveAll(source)
	}
	destination := filepath.Clean(request.Destination)
	if strings.TrimSpace(request.Destination) == "" || filepath.IsAbs(destination) || destination == "." || destination == ".." || strings.HasPrefix(destination, ".."+string(filepath.Separator)) {
		return errors.New("invalid destination")
	}
	parent := filepath.Dir(destination)
	var targetParent string
	if parent == "." {
		targetParent = root
	} else {
		targetParent, err = safeWorkspacePath(root, parent)
		if err != nil {
			return err
		}
	}
	target := filepath.Join(targetParent, filepath.Base(destination))
	if target == source || strings.HasPrefix(target, source+string(filepath.Separator)) {
		return errors.New("destination is inside source")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("destination already exists or is unavailable")
	}
	if request.Action == "rename" || request.Action == "move" {
		return os.Rename(source, target)
	}
	if err := copyProjectEntry(source, target); err != nil {
		return err
	}
	return nil
}

func copyProjectEntry(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("copying symbolic links is not supported")
	}
	if info.IsDir() {
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err == nil {
			for _, entry := range entries {
				if err = copyProjectEntry(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
					break
				}
			}
		}
		if err != nil {
			_ = os.RemoveAll(target)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("unsupported file type")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(target)
		return errors.Join(copyErr, closeErr)
	}
	return nil
}
