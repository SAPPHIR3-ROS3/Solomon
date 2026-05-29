package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
)

func AddRule(d Deps, parts []string) error {
	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return fmt.Errorf("usage: /add rule <phrase>")
	}
	n, err := instructions.AddRule(instructions.ScopeGlobal, "", text)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "global rule %d saved", n)
	return nil
}

func AddProjectRule(d Deps, parts []string) error {
	if d.ProjHex == "" {
		return fmt.Errorf("add projectrule: missing project id")
	}
	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return fmt.Errorf("usage: /add projectrule <phrase>")
	}
	n, err := instructions.AddRule(instructions.ScopeProject, d.ProjHex, text)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "project rule %d saved", n)
	return nil
}

func RemoveRule(d Deps, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("usage: /remove rule <N>")
	}
	n, err := instructions.ParseRuleNumber(parts[1])
	if err != nil {
		return err
	}
	if err := instructions.RemoveRule(instructions.ScopeGlobal, "", n); err != nil {
		return err
	}
	PrintSystemf(d.Out, "global rule %d removed (rules renumbered)", n)
	return nil
}

func RemoveProjectRule(d Deps, parts []string) error {
	if d.ProjHex == "" {
		return fmt.Errorf("remove projectrule: missing project id")
	}
	if len(parts) < 2 {
		return fmt.Errorf("usage: /remove projectrule <N>")
	}
	n, err := instructions.ParseRuleNumber(parts[1])
	if err != nil {
		return err
	}
	if err := instructions.RemoveRule(instructions.ScopeProject, d.ProjHex, n); err != nil {
		return err
	}
	PrintSystemf(d.Out, "project rule %d removed (rules renumbered)", n)
	return nil
}

func Rules(d Deps) error {
	return instructions.WriteRulesList(d.Out, d.ProjHex)
}
