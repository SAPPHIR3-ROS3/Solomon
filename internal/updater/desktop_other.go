//go:build !windows && !linux && !darwin

package updater

import (
	"context"
	"fmt"
	"io"
)

func registerDesktop(context.Context, string, string, string, io.Writer) error {
	return fmt.Errorf("desktop is supported on Windows, macOS and Linux")
}
