package server

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *projectAPI) handleEditorBaseline(w http.ResponseWriter, r *http.Request, root, target string) {
	content, isRepo, err := editorBaseline(r.Context(), root, target)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"content": content, "isRepo": isRepo})
}

func editorBaseline(parent context.Context, root, target string) (string, bool, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false, err
	}
	root = resolved
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	git := func(args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		configureManagedProcess(command)
		return command.Output()
	}
	inside, err := git("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(inside)) != "true" {
		return "", false, nil
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", true, err
	}
	prefix, err := git("rev-parse", "--show-prefix")
	if err != nil {
		return "", true, err
	}
	path := strings.TrimSuffix(string(prefix), "\n") + filepath.ToSlash(relative)
	if _, err = git("rev-parse", "--verify", "HEAD"); err != nil {
		return "", true, nil
	}
	object := "HEAD:" + path
	sizeBytes, err := git("cat-file", "-s", object)
	if err != nil {
		tracked, listErr := git("ls-tree", "--name-only", "HEAD", "--", filepath.ToSlash(relative))
		if listErr == nil && len(tracked) == 0 {
			return "", true, nil
		}
		return "", true, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeBytes)), 10, 64)
	if err != nil {
		return "", true, err
	}
	if size > 2<<20 {
		return "", true, errors.New("Git baseline exceeds the editor diff limit")
	}
	content, err := git("cat-file", "blob", object)
	if err != nil {
		return "", true, err
	}
	if strings.ContainsRune(string(content), 0) {
		return "", false, nil
	}
	return string(content), true, nil
}
