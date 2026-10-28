package app

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"

// ReadDroppedImages supplies bytes for native file-manager drops that expose
// local URIs or paths rather than browser File objects.
func (b ServerBridge) ReadDroppedImages(paths []string) ([]clipboard.DroppedImage, error) {
	return clipboard.ReadDroppedImages(paths)
}
