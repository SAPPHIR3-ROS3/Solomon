package plan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
)

func ResolvePath(plansDir, name string) (string, error) {
	fn, err := project.NormalizePlanName(name)
	if err != nil {
		return "", err
	}
	p := filepath.Join(plansDir, fn)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(plansDir)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, base+string(os.PathSeparator)) && abs != base {
		return "", os.ErrInvalid
	}
	return abs, nil
}

func CountPending(plansDir string) (int, error) {
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(plansDir, e.Name()))
		if err != nil {
			continue
		}
		meta, _, err := ParseDocument(b)
		if err != nil {
			continue
		}
		if meta.Status == StatusNotBuilt || meta.Status == StatusPartiallyBuilt {
			n++
		}
	}
	return n, nil
}

func ReadFile(path string) (Meta, Sections, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, Sections{}, nil, err
	}
	meta, body, err := ParseDocument(b)
	if err != nil {
		return Meta{}, Sections{}, nil, err
	}
	return meta, ParseSections(body), b, nil
}

func WriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}
