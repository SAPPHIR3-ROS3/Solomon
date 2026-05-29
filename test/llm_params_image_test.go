package test

import (
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestBuildUserContentPartsOmitsStaleImgTag(t *testing.T) {
	parts := llm.BuildUserContentParts("[img-0]", nil)
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want single text part, got %+v", parts)
	}
	if got := parts[0].GetText(); got == nil || *got != "" {
		t.Fatalf("want empty visible text without [img-0], got %q", *got)
	}
	parts = llm.BuildUserContentParts("pre [img-1] suf", map[int]string{1: filepath.Join(t.TempDir(), "nope.png")})
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want flattened text-only, got %+v", parts)
	}
}
