package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pathglob"
)

func TestPathglobNormalizePattern(t *testing.T) {
	if got := pathglob.NormalizePattern("*.go"); got != "**/*.go" {
		t.Fatalf("got %q", got)
	}
	if got := pathglob.NormalizePattern("**/*.go"); got != "**/*.go" {
		t.Fatalf("got %q", got)
	}
}

func TestPathglobMatch(t *testing.T) {
	cases := []struct {
		path, pat string
		want      bool
	}{
		{"internal/foo.go", "**/*.go", true},
		{"foo.txt", "**/*.go", false},
		{"pkg/sub/x.go", "pkg/**/*.go", true},
		{"other/x.go", "pkg/**/*.go", false},
		{"a.go", "*.go", true},
		{"dir/a.go", "*.go", false},
	}
	for _, c := range cases {
		ok, err := pathglob.Match(c.path, c.pat)
		if err != nil {
			t.Fatalf("%s %s: %v", c.path, c.pat, err)
		}
		if ok != c.want {
			t.Fatalf("%s %s: got %v want %v", c.path, c.pat, ok, c.want)
		}
	}
}
