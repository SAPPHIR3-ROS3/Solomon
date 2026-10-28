package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

type DroppedImage struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

// ReadDroppedImages reads local images explicitly supplied by a desktop drop.
// Return actual image data, never a path that the WebView cannot load.
func ReadDroppedImages(paths []string) ([]DroppedImage, error) {
	const maxImageBytes = 5 << 20
	const maxTotalBytes = 20 << 20 // Leaves room for base64 in the 32 MiB message request.
	var result []DroppedImage
	total := 0
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("dropped image path must be absolute")
		}
		data, err := readDroppedImage(path, maxImageBytes)
		if err != nil {
			return nil, fmt.Errorf("read dropped image %s: %w", filepath.Base(path), err)
		}
		total += len(data)
		if total > maxTotalBytes {
			return nil, fmt.Errorf("dropped images exceed the 20 MiB total limit")
		}
		mime, ok := images.MIMEForBinary(data)
		if !ok {
			return nil, fmt.Errorf("unsupported image content in %s", filepath.Base(path))
		}
		result = append(result, DroppedImage{Name: filepath.Base(path), Data: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)})
	}
	return result, nil
}

func readDroppedImage(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image exceeds the 5 MiB limit")
	}
	return data, nil
}
