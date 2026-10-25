package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func Activate(d Deps, parts []string) error {
	return setActivation(d, parts, true)
}

func Deactivate(d Deps, parts []string) error {
	return setActivation(d, parts, false)
}

func setActivation(d Deps, parts []string, enabled bool) error {
	if len(parts) == 0 {
		return activationUsage()
	}
	kind := strings.ToLower(strings.TrimSpace(parts[0]))
	switch kind {
	case "rule", "projectrule":
		if len(parts) != 2 {
			return activationUsage()
		}
		number, err := instructions.ParseRuleNumber(parts[1])
		if err != nil {
			return err
		}
		scope, projHex := instructions.ScopeGlobal, ""
		if kind == "projectrule" {
			if d.ProjHex == "" {
				return fmt.Errorf("%s: missing project id", kind)
			}
			scope, projHex = instructions.ScopeProject, d.ProjHex
		}
		changed, err := instructions.SetRuleEnabled(scope, projHex, number, enabled)
		if err != nil {
			return err
		}
		state := "already active"
		if !enabled {
			state = "already inactive"
		}
		if changed {
			state = "activated"
			if !enabled {
				state = "deactivated"
			}
		}
		PrintSystemf(d.Out, "%s rule %d %s", scope, number, state)
		return nil
	case "skill":
		name, scope, err := parseSkillActivation(parts[1:])
		if err != nil {
			return err
		}
		changed, err := skills.SetSkillDisabled(name, scope, d.ProjHex, d.ProjRoot, !enabled)
		if err != nil {
			return err
		}
		if d.MutateSession != nil {
			d.MutateSession(func(session *chatstore.Session) {
				if session != nil {
					session.Capabilities.SetSkillEnabled(name, enabled)
				}
			})
			if d.PersistSession != nil {
				if err := d.PersistSession(); err != nil {
					return err
				}
			}
		}
		state := "already active"
		if !enabled {
			state = "already inactive"
		}
		if changed {
			state = "activated"
			if !enabled {
				state = "deactivated"
			}
		}
		PrintSystemf(d.Out, "skill %q %s", name, state)
		return nil
	default:
		return activationUsage()
	}
}

func parseSkillActivation(parts []string) (string, string, error) {
	if len(parts) == 0 {
		return "", "", activationUsage()
	}
	scope := ""
	nameParts := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		if strings.EqualFold(strings.TrimSpace(parts[i]), "--scope") {
			if scope != "" || i+1 >= len(parts) || i+2 != len(parts) {
				return "", "", activationUsage()
			}
			scope = strings.ToLower(strings.TrimSpace(parts[i+1]))
			if scope != skills.ScopeGlobal && scope != skills.ScopeProject && scope != skills.ScopeLocal {
				return "", "", activationUsage()
			}
			i++
			continue
		}
		nameParts = append(nameParts, parts[i])
	}
	name := strings.TrimSpace(strings.Join(nameParts, " "))
	if name == "" {
		return "", "", activationUsage()
	}
	return name, scope, nil
}

func activationUsage() error {
	return fmt.Errorf("usage: /activate|/deactivate rule <N> | projectrule <N> | skill <name> [--scope global|project|local]")
}
