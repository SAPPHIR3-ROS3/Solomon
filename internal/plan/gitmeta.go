package plan

import (
	"os/exec"
	"path/filepath"
	"strings"
)

type GitMeta struct {
	CommitHash   string
	LastCommitAt string
}

func GitMetaFromRoot(projRoot string) GitMeta {
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return GitMeta{}
	}
	hash, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(hash) == "" {
		return GitMeta{}
	}
	date, _ := gitOutput(root, "log", "-1", "--format=%cI", "HEAD")
	return GitMeta{
		CommitHash:   strings.TrimSpace(hash),
		LastCommitAt: strings.TrimSpace(date),
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
