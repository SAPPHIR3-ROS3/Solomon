package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

func CloneOrPull(ctx context.Context, remoteURL, destDir string) error {
	fi, err := os.Stat(destDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
			return err
		}
		_, err = git.PlainCloneContext(ctx, destDir, false, &git.CloneOptions{
			URL:      remoteURL,
			Progress: nil,
		})
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", destDir)
	}
	r, err := git.PlainOpen(destDir)
	if err != nil {
		return fmt.Errorf("open git repo: %w", err)
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	err = w.PullContext(ctx, &git.PullOptions{RemoteName: git.DefaultRemoteName})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}
