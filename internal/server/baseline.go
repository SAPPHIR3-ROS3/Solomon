package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *projectAPI) handleEditorBaseline(w http.ResponseWriter, r *http.Request, root, target string) {
	content, isRepo, err := editorBaseline(r.Context(), root, target)
	if err != nil {
		if r.Context().Err() != nil {
			logging.Diagnostic("editor baseline request canceled", map[string]any{"request_id": logging.RequestID(r.Context()), "outcome": "canceled"})
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeAPIError(w, http.StatusGatewayTimeout, errors.New("Git baseline timed out after 5 seconds"))
			return
		}
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
	git := func(expectedExit int, args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		command.Env = append(os.Environ(), "LC_ALL=C", "GIT_LITERAL_PATHSPECS=1")
		configureManagedProcess(command)
		started := time.Now()
		output, err := command.Output()
		var exit *exec.ExitError
		expected := expectedExit >= 0 && ctx.Err() == nil && errors.As(err, &exit) && exit.ExitCode() == expectedExit
		if expectedExit == 128 && expected {
			expected = strings.Contains(string(exit.Stderr), "not a git repository")
		}
		if !expected {
			logging.ProcessFinished(ctx, command, "editor.baseline."+args[0], started, err)
		}
		if ctx.Err() != nil {
			return output, ctx.Err()
		}
		if expected {
			return nil, nil
		}
		return output, err
	}
	inside, err := git(128, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return "", false, fmt.Errorf("discover Git baseline repository: %w", err)
	}
	if strings.TrimSpace(string(inside)) != "true" {
		return "", false, nil
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", true, err
	}
	prefix, err := git(-1, "rev-parse", "--show-prefix")
	if err != nil {
		return "", true, err
	}
	path := strings.TrimSuffix(string(prefix), "\n") + filepath.ToSlash(relative)
	tree, err := git(1, "rev-parse", "--verify", "--quiet", "HEAD^{tree}")
	if err != nil {
		return "", true, fmt.Errorf("resolve Git baseline tree: %w", err)
	}
	if len(tree) == 0 {
		return "", true, nil
	}
	entries, err := git(-1, "ls-tree", "--full-name", "--long", "-z", strings.TrimSpace(string(tree)), "--", filepath.ToSlash(relative))
	if err != nil {
		return "", true, fmt.Errorf("list Git baseline entry: %w", err)
	}
	object := ""
	for _, entry := range strings.Split(string(entries), "\x00") {
		metadata, name, ok := strings.Cut(entry, "\t")
		if !ok || name != path {
			continue
		}
		fields := strings.Fields(metadata)
		if len(fields) != 4 || fields[1] != "blob" {
			return "", false, nil
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return "", true, err
		}
		if size > 2<<20 {
			return "", true, errors.New("Git baseline exceeds the editor diff limit")
		}
		object = fields[2]
		break
	}
	if object == "" {
		return "", true, nil
	}
	content, err := git(-1, "cat-file", "blob", object)
	if err != nil {
		return "", true, fmt.Errorf("read Git baseline blob: %w", err)
	}
	if strings.ContainsRune(string(content), 0) {
		return "", false, nil
	}
	return string(content), true, nil
}
