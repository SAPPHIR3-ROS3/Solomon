//go:build ignore

// check_doc_paths verifies markdown links and cited repo paths in docs/, README.md, and TODO.md.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	linkPathRE   = regexp.MustCompile(`\]\(([^)]+)\)`)
	backtickRE   = regexp.MustCompile("`((?:internal|cmd|integrations|test)/[^`]+(?:\\.go|/))`")
	inlinePathRE = regexp.MustCompile("(?:^|[\\s|])((?:internal|cmd|integrations|test)/[\\w./_-]+\\.go)")
	headingRE    = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	spaceRE      = regexp.MustCompile(`\s+`)
)

func shouldCheckPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "*") {
		return false
	}
	if strings.HasSuffix(raw, "/") {
		return true
	}
	ext := filepath.Ext(raw)
	switch ext {
	case ".go", ".md", ".yml", ".yaml", ".toml", ".json":
		return true
	default:
		return false
	}
}

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check_doc_paths: %v\n", err)
		os.Exit(2)
	}

	var files []string
	_ = filepath.Walk(filepath.Join(root, "docs"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	for _, name := range []string{"README.md", "TODO.md"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}

	anchorCache := map[string]map[string]struct{}{}
	var bad []string
	seen := map[string]struct{}{}

	for _, doc := range files {
		data, err := os.ReadFile(doc)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: read error: %v", rel(root, doc), err))
			continue
		}
		text := string(data)
		docDir := filepath.Dir(doc)
		isTODO := filepath.Base(doc) == "TODO.md"

		addPath := func(raw string) {
			raw = strings.TrimSpace(raw)
			if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "mailto:") {
				return
			}
			if !shouldCheckPath(raw) {
				return
			}
			pathPart := strings.Split(raw, "#")[0]
			if pathPart == "" {
				return
			}
			key := rel(root, doc) + " -> " + raw
			if _, ok := seen[key]; ok {
				return
			}
			seen[key] = struct{}{}

			target, ok := resolvePath(root, docDir, pathPart)
			if !ok {
				return
			}
			if _, err := os.Stat(target); err != nil {
				bad = append(bad, fmt.Sprintf("%s: missing %s", rel(root, doc), rel(root, target)))
			}
		}

		for _, m := range linkPathRE.FindAllStringSubmatch(text, -1) {
			raw := strings.TrimSpace(m[1])
			if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "mailto:") {
				continue
			}
			pathPart, fragment := splitFragment(raw)
			if pathPart == "" {
				if fragment != "" {
					checkAnchor(root, doc, doc, fragment, anchorCache, &bad, &seen)
				}
				continue
			}
			if !shouldCheckPath(pathPart) {
				continue
			}
			key := rel(root, doc) + " -> " + raw
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			target, ok := resolvePath(root, docDir, pathPart)
			if !ok {
				continue
			}
			if _, err := os.Stat(target); err != nil {
				bad = append(bad, fmt.Sprintf("%s: missing %s", rel(root, doc), rel(root, target)))
				continue
			}
			if fragment != "" && strings.HasSuffix(strings.ToLower(pathPart), ".md") {
				checkAnchor(root, doc, target, fragment, anchorCache, &bad, &seen)
			}
		}
		for _, m := range backtickRE.FindAllStringSubmatch(text, -1) {
			if !isTODO {
				addPath(m[1])
			}
		}
		for _, m := range inlinePathRE.FindAllStringSubmatch(text, -1) {
			if !isTODO {
				addPath(m[1])
			}
		}
	}

	if len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "check_doc_paths: %d broken link(s):\n", len(bad))
		for _, line := range bad {
			fmt.Fprintf(os.Stderr, "  %s\n", line)
		}
		os.Exit(1)
	}
	fmt.Println("check_doc_paths: ok")
}

func splitFragment(raw string) (path, fragment string) {
	i := strings.Index(raw, "#")
	if i < 0 {
		return raw, ""
	}
	return raw[:i], raw[i+1:]
}

func resolvePath(root, docDir, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if filepath.IsAbs(raw) {
		return raw, true
	}
	if strings.HasPrefix(raw, "internal/") || strings.HasPrefix(raw, "cmd/") ||
		strings.HasPrefix(raw, "integrations/") || strings.HasPrefix(raw, "test/") ||
		strings.HasPrefix(raw, "docs/") || strings.HasPrefix(raw, "scripts/") {
		return filepath.Clean(filepath.Join(root, raw)), true
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		return filepath.Clean(filepath.Join(docDir, raw)), true
	}
	return filepath.Clean(filepath.Join(docDir, raw)), true
}

func checkAnchor(root, fromDoc, targetDoc, fragment string, cache map[string]map[string]struct{}, bad *[]string, seen *map[string]struct{}) {
	fragment = strings.ToLower(strings.TrimSpace(fragment))
	if fragment == "" {
		return
	}
	key := rel(root, fromDoc) + " -> #" + fragment + " in " + rel(root, targetDoc)
	if _, ok := (*seen)[key]; ok {
		return
	}
	(*seen)[key] = struct{}{}

	abs := targetDoc
	anchors, err := loadAnchors(abs, cache)
	if err != nil {
		*bad = append(*bad, fmt.Sprintf("%s: read anchor target %s: %v", rel(root, fromDoc), rel(root, abs), err))
		return
	}
	if _, ok := anchors[fragment]; ok {
		return
	}
	norm := normalizeAnchor(fragment)
	for id := range anchors {
		if normalizeAnchor(id) == norm {
			return
		}
	}
	*bad = append(*bad, fmt.Sprintf("%s: missing anchor #%s in %s", rel(root, fromDoc), fragment, rel(root, abs)))
}

func normalizeAnchor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func loadAnchors(path string, cache map[string]map[string]struct{}) (map[string]struct{}, error) {
	if a, ok := cache[path]; ok {
		return a, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	anchors := map[string]struct{}{}
	for _, m := range headingRE.FindAllStringSubmatch(string(data), -1) {
		id := headingAnchor(m[1])
		if id != "" {
			anchors[id] = struct{}{}
		}
	}
	cache[path] = anchors
	return anchors, nil
}

func headingAnchor(title string) string {
	title = strings.TrimSpace(title)
	title = strings.ReplaceAll(title, "`", "")
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' {
			b.WriteRune(r)
		}
	}
	s := spaceRE.ReplaceAllString(strings.TrimSpace(b.String()), "-")
	return strings.Trim(s, "-")
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}
