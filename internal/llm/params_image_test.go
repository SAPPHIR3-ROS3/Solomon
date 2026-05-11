package llm

import (
	"path/filepath"
	"testing"
)

func TestBuildUserContentPartsOmitsStaleImgTag(t *testing.T) {
	parts := buildUserContentParts("[img-0]", nil)
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want single text part, got %+v", parts)
	}
	if got := parts[0].GetText(); got == nil || *got != "" {
		t.Fatalf("want empty visible text without [img-0], got %q", *got)
	}
	parts = buildUserContentParts("pre [img-1] suf", map[int]string{1: filepath.Join(t.TempDir(), "nope.png")})
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want flattened text-only, got %+v", parts)
	}
}
