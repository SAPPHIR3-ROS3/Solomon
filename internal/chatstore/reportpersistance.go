package chatstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func ResearchDir(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "research"), nil
}

func EnsureResearchDir(projectHex string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func ResearchJobPath(projectHex, slug string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", os.ErrInvalid
	}
	return filepath.Join(d, slug+".json"), nil
}

func ResearchHTMLPath(projectHex, slug string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", os.ErrInvalid
	}
	return filepath.Join(d, slug+".html"), nil
}

func WriteResearchJobFile(projectHex string, slug string, v any) error {
	p, err := ResearchJobPath(projectHex, slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func DeleteResearchJob(projectHex, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return os.ErrInvalid
	}
	jsonPath, err := ResearchJobPath(projectHex, slug)
	if err != nil {
		return err
	}
	htmlPath, err := ResearchHTMLPath(projectHex, slug)
	if err != nil {
		return err
	}
	var firstErr error
	if err := os.Remove(jsonPath); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if err := os.Remove(htmlPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func ReadResearchJobFile(projectHex, slug string, dest any) error {
	p, err := ResearchJobPath(projectHex, slug)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func ListResearchJobFiles(projectHex string) ([]string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			slugs = append(slugs, strings.TrimSuffix(name, ".json"))
		}
	}
	return slugs, nil
}
