package plan

import (
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	started := time.Now()
	out, err := cmd.Output()
	logging.ProcessFinished(context.Background(), cmd, "plan.git."+args[0], started, err)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
