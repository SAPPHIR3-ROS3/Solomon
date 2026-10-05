package updater

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func registerDesktop(ctx context.Context, tag, cli, target string, progress io.Writer) error {
	// The app's server launcher uses the CLI beside the GUI binary. A symlink
	// keeps it current when go install replaces the CLI on a subsequent update.
	link := filepath.Join(filepath.Dir(target), "solomon")
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(cli, link); err != nil {
		return err
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(target)))
	cmd := exec.CommandContext(ctx, "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister", "-f", bundle)
	cmd.Stdout, cmd.Stderr = progress, progress
	return cmd.Run()
}
