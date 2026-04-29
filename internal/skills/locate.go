package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LocateSkillDir(repoRoot string, preferredSkill string) (relDir string, mdAbs string, err error) {
	repoRoot = filepath.Clean(repoRoot)
	rootMD := filepath.Join(repoRoot, "SKILL.md")
	if st, e := os.Stat(rootMD); e == nil && !st.IsDir() {
		return ".", rootMD, nil
	}
	preferredSkill = strings.TrimSpace(preferredSkill)
	if preferredSkill != "" {
		candidates := []string{
			filepath.Join(repoRoot, "skills", preferredSkill, "SKILL.md"),
			filepath.Join(repoRoot, preferredSkill, "SKILL.md"),
		}
		for _, p := range candidates {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				dir := filepath.Dir(p)
				rel, e2 := filepath.Rel(repoRoot, dir)
				if e2 != nil {
					return "", "", e2
				}
				return filepathToSlash(rel), p, nil
			}
		}
	}
	var found []string
	_ = filepath.WalkDir(repoRoot, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(p)
			if base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Base(p), "SKILL.md") {
			found = append(found, p)
		}
		return nil
	})
	if len(found) == 1 {
		dir := filepath.Dir(found[0])
		rel, e2 := filepath.Rel(repoRoot, dir)
		if e2 != nil {
			return "", "", e2
		}
		return filepathToSlash(rel), found[0], nil
	}
	if len(found) == 0 {
		return "", "", fmt.Errorf("SKILL.md not found under %s", repoRoot)
	}
	if preferredSkill != "" {
		for _, p := range found {
			if strings.Contains(strings.ToLower(p), strings.ToLower(preferredSkill)) {
				dir := filepath.Dir(p)
				rel, e2 := filepath.Rel(repoRoot, dir)
				if e2 != nil {
					return "", "", e2
				}
				return filepathToSlash(rel), p, nil
			}
		}
	}
	return "", "", fmt.Errorf("multiple SKILL.md files; specify a skills.sh page with --skill or use a repo with a single SKILL.md")
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
