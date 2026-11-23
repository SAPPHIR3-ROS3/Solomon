package clipboard

import (
	"context"
	"fmt"
	"os"
	"runtime"
)

// ReadImage reads image bytes without changing the clipboard.
func ReadImage(ctx context.Context) ([]byte, error) {
	if runtime.GOOS == "linux" {
		return readLinuxImage(ctx)
	}
	if err := ImageStatus(); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "solomon-clipboard-")
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary clipboard directory: %w. Check temporary directory permissions and available disk space", err)
	}
	defer os.RemoveAll(dir)
	path, err := PasteImage(dir, "paste", 0)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the temporary clipboard image: %w. Check temporary directory permissions or attach the image by file path", err)
	}
	return data, nil
}
