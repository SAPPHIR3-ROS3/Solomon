package skills

import (
	"fmt"
	"strings"
)

func displayNameUsedByOther(r *Registry, name string, scope, projHex, skillKey string) bool {
	if r == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for k, e := range r.Global {
		if nameMatches(e.Name, name) {
			if scope == ScopeGlobal && k == skillKey {
				continue
			}
			return true
		}
	}
	for ph, m := range r.Projects {
		for k, e := range m {
			if !nameMatches(e.Name, name) {
				continue
			}
			if (scope == ScopeProject || scope == ScopeLocal) && ph == projHex && k == skillKey {
				continue
			}
			return true
		}
	}
	return false
}

func UniqueDisplayName(r *Registry, canonical, baseDisplay, scope, projHex, skillKey string) string {
	baseDisplay = strings.TrimSpace(baseDisplay)
	if baseDisplay == "" {
		baseDisplay = "skill"
	}
	if !displayNameUsedByOther(r, baseDisplay, scope, projHex, skillKey) {
		return baseDisplay
	}
	owner := RepoOwner(canonical)
	if owner == "" {
		owner = "repo"
	}
	candidate := owner + "-" + baseDisplay
	if !displayNameUsedByOther(r, candidate, scope, projHex, skillKey) {
		return candidate
	}
	for n := 2; n < 10_000; n++ {
		candidate = fmt.Sprintf("%s-%s-%d", owner, baseDisplay, n)
		if !displayNameUsedByOther(r, candidate, scope, projHex, skillKey) {
			return candidate
		}
	}
	suffix := skillKey
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	return owner + "-" + suffix + "-name"
}
