package anthropic

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"testing"
)

func TestDisplayOnlyMediaExcludedFromVisionBlocks(t *testing.T) {
	a := images.Attachment{Path: "video.mp4", MIMEType: "image/png", Data: []byte{1, 2, 3}, PresentationOnly: true}
	if len(toolImageBlocks([]images.Attachment{a})) != 0 {
		t.Fatal("presentation preview became model image")
	}
	a.PresentationOnly = false
	if len(toolImageBlocks([]images.Attachment{a})) != 2 {
		t.Fatal("explicit vision image lost")
	}
}
