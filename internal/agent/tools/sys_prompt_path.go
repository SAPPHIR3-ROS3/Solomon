package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func ResolveSysPromptPath(projRoot, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	tplDir, tplErr := paths.PromptTemplatesDir()
	if tplErr == nil {
		for _, c := range sysPromptTemplateCandidates(tplDir, p) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	if tplErr == nil && (strings.HasSuffix(strings.ToLower(p), ".tmpl") || isBareSolomonTemplateName(p)) {
		return filepath.Join(tplDir, ensureTmplSuffix(filepath.Base(p)))
	}
	return resolveProjectPath(projRoot, p)
}

func isBareSolomonTemplateName(p string) bool {
	if strings.Contains(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	if strings.Contains(filepath.Base(p), ".") {
		return strings.HasSuffix(strings.ToLower(p), ".tmpl")
	}
	return true
}

func ensureTmplSuffix(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".tmpl") {
		return name
	}
	return name + ".tmpl"
}

func sysPromptTemplateCandidates(tplDir, p string) []string {
	base := filepath.Base(p)
	cands := []string{filepath.Join(tplDir, p)}
	if p != base {
		cands = append(cands, filepath.Join(tplDir, base))
	}
	if !strings.HasSuffix(strings.ToLower(p), ".tmpl") {
		cands = append(cands, filepath.Join(tplDir, p+".tmpl"))
		if p != base {
			cands = append(cands, filepath.Join(tplDir, base+".tmpl"))
		}
	}
	return cands
}
