package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
)

func TestDocumentTagsMarkdownCode(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"language fence", "```tsx\nimport Button from '@mui/material/Button'\nconst re = /@([^\\s@]+)/\n```\n@docs/file.md"},
		{"tilde fence", "~~~javascript\nimport('@vercel/analytics/react')\n~~~\n@docs/file.md"},
		{"long fence", "````typescript\n```\n@ignored\n~~~\n@ignored\n````\n@docs/file.md"},
		{"longer closing fence", "```go\n@ignored\n`````\n@docs/file.md"},
		{"closing fence with info", "```tsx\n```javascript\n@ignored\n```\n@docs/file.md"},
		{"indented fence", "  ```tsx\n@ignored\n  ```\n@docs/file.md"},
		{"inline code", "`@ignored` @docs/file.md"},
		{"double backticks", "``before ` @ignored ` after`` @docs/file.md"},
		{"different inline delimiter lengths", "`before `` @ignored `` after` @docs/file.md"},
		{"multiline inline code", "``before\n` @ignored\nafter`` @docs/file.md"},
		{"unmatched backtick", "unmatched ` @docs/file.md"},
		{"plain inclusion", "include @docs/file.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tags := atmention.FindDocumentTags(tc.text)
			if len(tags) != 1 || tags[0].Path != "docs/file.md" {
				t.Fatalf("unexpected tags: %#v", tags)
			}
			tag := tags[0]
			if tc.text[tag.Start:tag.End] != "@docs/file.md" {
				t.Fatalf("incorrect offsets: %#v", tag)
			}
		})
	}
}

func TestDocumentTagsUnclosedFence(t *testing.T) {
	tags := atmention.FindDocumentTags("@docs/file.md\n```tsx\n@ignored")
	if len(tags) != 1 || tags[0].Path != "docs/file.md" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestExpandDocumentIgnoresCodeImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child.md"), []byte("included body"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "```tsx\nimport Button from '@mui/material/Button'\n```\n``@lib/auth ` @ignored``\n@child.md"
	notifier := atmention.NewNotifier()
	got, err := atmention.ExpandDocument(context.Background(), source, filepath.Join(dir, "SKILL.md"), dir, notifier)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, source, "--- file child.md ---", "included body") {
		t.Fatalf("unexpected document: %q", got)
	}
	if messages := notifier.Messages(); len(messages) != 0 {
		t.Fatalf("code generated inclusion warnings: %#v", messages)
	}
}
