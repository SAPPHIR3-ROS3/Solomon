//go:build !windows

package updater

import (
	"context"
	"io"
)

func runWindowsProfileSetup(context.Context, io.Writer) error {
	return nil
}

func windowsProfileSetupScriptLines() string {
	return ""
}
