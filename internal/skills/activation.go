package skills

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type skillStateTarget struct {
	key   string
	scope string
	entry SkillEntry
}

func SetSkillDisabled(name, scope, projHex, projRoot string, disabled bool) (bool, error) {
	name = strings.TrimSpace(name)
	scope = strings.ToLower(strings.TrimSpace(scope))
	if name == "" {
		return false, fmt.Errorf("skill name is empty")
	}
	if scope != "" && scope != ScopeGlobal && scope != ScopeProject && scope != ScopeLocal {
		return false, fmt.Errorf("unknown skill scope %q", scope)
	}
	if (scope == ScopeProject || scope == ScopeLocal) && (strings.TrimSpace(projHex) == "" || strings.TrimSpace(projRoot) == "") {
		return false, fmt.Errorf("project skill scope requires a current project")
	}
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return false, err
	}
	lockPath, err := paths.SkillsRegistryLockPath()
	if err != nil {
		return false, err
	}
	changed := false
	err = WithRegistryLock(lockPath, regPath, func(reg *Registry) error {
		var matches []skillStateTarget
		if scope == "" || scope == ScopeGlobal {
			for key, entry := range reg.Global {
				if nameMatches(entry.Name, name) {
					matches = append(matches, skillStateTarget{key: key, scope: ScopeGlobal, entry: entry})
				}
			}
		}
		if projHex != "" && (scope == "" || scope == ScopeProject || scope == ScopeLocal) {
			for key, entry := range reg.Projects[projHex] {
				local := projRoot != "" && clonePathUnderLocal(entry, projRoot)
				if scope == ScopeLocal && !local {
					continue
				}
				if scope == ScopeProject && local {
					continue
				}
				if nameMatches(entry.Name, name) {
					kind := ScopeProject
					if local {
						kind = ScopeLocal
					}
					matches = append(matches, skillStateTarget{key: key, scope: kind, entry: entry})
				}
			}
		}
		if len(matches) == 0 {
			return fmt.Errorf("no skill found with name %q in the selected scope", name)
		}
		if len(matches) > 1 {
			return fmt.Errorf("multiple skills named %q; specify --scope global, project, or local", name)
		}
		target := matches[0]
		if target.entry.Disabled == disabled {
			return nil
		}
		target.entry.Disabled = disabled
		if target.scope == ScopeGlobal {
			reg.Global[target.key] = target.entry
		} else {
			reg.Projects[projHex][target.key] = target.entry
			mirror := paths.LocalSkillsMirrorPath(projRoot)
			if err := SaveMirrorJSON(mirror, ProjectEntries(reg, projHex)); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}
