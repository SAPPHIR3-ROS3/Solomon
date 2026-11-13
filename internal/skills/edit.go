package skills

import (
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func GlobalMarkdown(id string, content *string, remove bool) (string, error) {
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return "", err
	}
	lockPath, err := paths.SkillsRegistryLockPath()
	if err != nil {
		return "", err
	}
	var result string
	err = WithRegistryLock(lockPath, registryPath, func(reg *Registry) error {
		entry, ok := reg.Global[id]
		if !ok {
			return fmt.Errorf("skill not found")
		}
		root, err := paths.GlobalSkillsDir()
		if err != nil {
			return err
		}
		if !relUnderResolved(entry.ClonePath, root) || resolveExistingPath(entry.ClonePath) == resolveExistingPath(root) {
			return fmt.Errorf("invalid skill directory")
		}
		if remove {
			for key, other := range reg.Global {
				if key != id && other.ClonePath == entry.ClonePath {
					return fmt.Errorf("skill directory is shared")
				}
			}
			for _, entries := range reg.Projects {
				for _, other := range entries {
					if other.ClonePath == entry.ClonePath {
						return fmt.Errorf("skill directory is shared")
					}
				}
			}
			if err := os.RemoveAll(entry.ClonePath); err != nil {
				return err
			}
			delete(reg.Global, id)
			return nil
		}
		if !relUnderResolved(entry.SkillMdPath, entry.ClonePath) {
			return fmt.Errorf("invalid skill file")
		}
		if content == nil {
			raw, err := os.ReadFile(entry.SkillMdPath)
			result = string(raw)
			return err
		}
		if strings.TrimSpace(*content) == "" {
			return fmt.Errorf("skill content is required")
		}
		fm, body, err := ParseSkillBytes([]byte(*content))
		if err != nil {
			return err
		}
		name := DisplayNameFromFrontMatter(fm, entry.Name)
		name = UniqueDisplayName(reg, entry.SourceRepo, name, ScopeGlobal, "", id)
		if err := WriteSkillMarkdown(entry.SkillMdPath, fm, body); err != nil {
			return err
		}
		entry.Name, entry.FrontMatter = name, fm
		reg.Global[id] = entry
		return nil
	})
	return result, err
}
