//go:build !linux || !cgo || (!dev && !production) || bindings

package app

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
)

func readDesktopClipboardImage(ctx context.Context) ([]byte, error) {
	return clipboard.ReadImage(ctx)
}
