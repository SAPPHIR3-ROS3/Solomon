package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (a *projectAPI) handleProjectCommit(w http.ResponseWriter, r *http.Request, projectID string) {
	var request struct {
		Message string `json:"message"`
	}
	if err := decodeJSONBody(w, r, &request, 1<<20); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	message := strings.TrimSpace(request.Message)
	if message == "" || strings.ContainsRune(message, 0) {
		writeAPIError(w, http.StatusBadRequest, errors.New("a nonempty commit message without NUL characters is required"))
		return
	}
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
	if !status.IsRepo || len(status.Staged) == 0 {
		writeAPIError(w, http.StatusConflict, errors.New("no staged changes to commit"))
		return
	}
	// Do not stage files or pass pathspecs: Git must commit exactly the index,
	// including staged versions of files that also have unstaged edits.
	// Allow hooks/signing time to finish, independently of a client disconnect.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", root, "commit", "-m", message)
	configureManagedProcess(command)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if ctx.Err() != nil {
			detail = "commit timed out; check Git history before retrying"
		} else if detail == "" {
			detail = err.Error()
		}
		writeAPIError(w, http.StatusConflict, fmt.Errorf("unable to commit staged changes: %s", detail))
		return
	}
	// A later refresh failure must not turn a successful commit into a failed
	// response that could cause the user to submit the commit again.
	writeJSON(w, http.StatusOK, map[string]bool{"committed": true})
}
