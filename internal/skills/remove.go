package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"solomon/internal/paths"
)

type RemoveOpts struct {
	Out      io.Writer
	ProjHex  string
	ProjRoot string
	Args     []string
}

func ParseRemoveArgs(parts []string) (string, error) {
	if len(parts) < 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "skill") {
		return "", fmt.Errorf(`usage: /remove skill <name> — removes all registry entries with this display name and deletes their clone directories`)
	}
	name := strings.TrimSpace(strings.Join(parts[1:], " "))
	if name == "" {
		return "", fmt.Errorf("remove: skill name is empty")
	}
	return name, nil
}

func RunRemove(opts RemoveOpts) error {
	name, err := ParseRemoveArgs(opts.Args)
	if err != nil {
		return err
	}
	if opts.ProjRoot == "" || opts.ProjHex == "" {
		return fmt.Errorf("remove: missing project root or id")
	}
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return err
	}
	lockPath, err := paths.SkillsRegistryLockPath()
	if err != nil {
		return err
	}
	var toDelete []string
	err = WithRegistryLock(lockPath, regPath, func(r *Registry) error {
		var seenPath = map[string]struct{}{}
		projTouched := map[string]struct{}{}
		toDelete = nil
		removed := 0
		for key, e := range r.Global {
			if nameMatches(e.Name, name) {
				if err := collectClonePath(e.ClonePath, opts.ProjRoot, seenPath, &toDelete); err != nil {
					return err
				}
				delete(r.Global, key)
				removed++
			}
		}
		for projID, m := range r.Projects {
			for key, e := range m {
				if nameMatches(e.Name, name) {
					if err := collectClonePath(e.ClonePath, opts.ProjRoot, seenPath, &toDelete); err != nil {
						return err
					}
					delete(m, key)
					removed++
					projTouched[projID] = struct{}{}
				}
			}
			if len(m) == 0 {
				delete(r.Projects, projID)
			}
		}
		if removed == 0 {
			return fmt.Errorf("no skill found with name %q", name)
		}
		if _, ok := projTouched[opts.ProjHex]; ok {
			mirror := paths.LocalSkillsMirrorPath(opts.ProjRoot)
			if err := SaveMirrorJSON(mirror, ProjectEntries(r, opts.ProjHex)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, p := range toDelete {
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("remove clone tree %s: %w", p, err)
		}
	}
	if opts.Out != nil {
		fmt.Fprintf(opts.Out, "Removed skill %q (%d clone dir(s)).\n", name, len(toDelete))
	}
	return nil
}

func collectClonePath(clonePath string, projRoot string, seen map[string]struct{}, out *[]string) error {
	p := strings.TrimSpace(clonePath)
	if p == "" {
		return nil
	}
	ap, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return err
	}
	if !pathIsUnderSolomonSkillRoots(ap, projRoot) {
		return fmt.Errorf("refusing to delete clone outside Solomon skill dirs: %s", p)
	}
	if _, ok := seen[ap]; !ok {
		seen[ap] = struct{}{}
		*out = append(*out, ap)
	}
	return nil
}

func nameMatches(regName, want string) bool {
	return strings.EqualFold(strings.TrimSpace(regName), strings.TrimSpace(want))
}

func pathIsUnderSolomonSkillRoots(absClone, projRoot string) bool {
	c, err := filepath.Abs(filepath.Clean(absClone))
	if err != nil {
		return false
	}
	if relUnderResolved(c, paths.LocalSkillsDir(projRoot)) {
		return true
	}
	globDir, err := paths.GlobalSkillsDir()
	if err == nil && relUnderResolved(c, globDir) {
		return true
	}
	projectsRoot, err := paths.ProjectsDir()
	if err != nil {
		return false
	}
	pr, err := filepath.Abs(filepath.Clean(projectsRoot))
	if err != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(pr); err == nil {
		pr = r
	}
	if r, err := filepath.EvalSymlinks(c); err == nil {
		c = r
	}
	rel, err := filepath.Rel(pr, c)
	if err != nil {
		return false
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) >= 3 && strings.EqualFold(parts[1], "skills") {
		return true
	}
	return false
}

func relUnderResolved(child, root string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = r
	}
	childAbs, err := filepath.Abs(filepath.Clean(child))
	if err != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(childAbs); err == nil {
		childAbs = r
	}
	rel, err := filepath.Rel(rootAbs, childAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
