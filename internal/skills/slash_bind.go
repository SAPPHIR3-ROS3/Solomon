package skills

import (
	"fmt"
	"sort"
	"strings"

	"solomon/internal/paths"
)

type SkillRefWithKey struct {
	RegistryKey string
	Entry       SkillEntry
}

type SkillSlashBinding struct {
	Slash string
	Entry SkillEntry
}

func ReservedSlashCommandNames() map[string]struct{} {
	return map[string]struct{}{
		"plan": {}, "build": {}, "clear": {}, "exec": {}, "log": {},
		"reasoning": {}, "timeout": {}, "stats": {}, "thinking": {},
		"max_response": {}, "threshold": {}, "models": {}, "connect": {},
		"new": {}, "resume": {}, "summarize": {}, "compact": {},
		"exit": {}, "quit": {}, "language": {}, "legacytools": {}, "legacy": {},
		"add": {}, "skills": {}, "remove": {}, "help": {}, "skill": {},
	}
}

func skillSlashBase(name string) string {
	return strings.ToLower(strings.TrimPrefix(SkillHelpCommand(name), "/"))
}

func sortRefsByNameKey(refs []SkillRefWithKey) {
	sort.Slice(refs, func(i, j int) bool {
		ni := strings.ToLower(strings.TrimSpace(refs[i].Entry.Name))
		nj := strings.ToLower(strings.TrimSpace(refs[j].Entry.Name))
		if ni != nj {
			return ni < nj
		}
		return refs[i].RegistryKey < refs[j].RegistryKey
	})
}

func orderedSkillRefs(r *Registry, projHex, projRoot string) []SkillRefWithKey {
	var locals, projects, globals []SkillRefWithKey
	for k, e := range r.Global {
		globals = append(globals, SkillRefWithKey{RegistryKey: k, Entry: e})
	}
	sortRefsByNameKey(globals)
	if projHex == "" {
		return globals
	}
	m := r.Projects[projHex]
	if m == nil {
		return globals
	}
	for k, e := range m {
		ref := SkillRefWithKey{RegistryKey: k, Entry: e}
		if projRoot != "" && clonePathUnderLocal(e, projRoot) {
			locals = append(locals, ref)
		} else if clonePathUnderProject(e, projHex) {
			projects = append(projects, ref)
		} else {
			projects = append(projects, ref)
		}
	}
	sortRefsByNameKey(locals)
	sortRefsByNameKey(projects)
	out := make([]SkillRefWithKey, 0, len(locals)+len(projects)+len(globals))
	out = append(out, locals...)
	out = append(out, projects...)
	out = append(out, globals...)
	return out
}

func pickSlashName(base, regKey string, res map[string]struct{}, claimed map[string]struct{}) string {
	canTake := func(s string) bool {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return false
		}
		if _, ok := res[s]; ok {
			return false
		}
		if _, ok := claimed[s]; ok {
			return false
		}
		return true
	}
	if base == "" {
		if len(regKey) >= 8 {
			base = regKey[:8]
		} else {
			base = regKey
		}
		if base == "" {
			base = "skill"
		}
	}
	candidates := []string{base, "skill-" + base}
	if regKey != "" {
		if len(regKey) >= 8 {
			candidates = append(candidates, fmt.Sprintf("skill-%s-%s", base, regKey[:8]))
		} else {
			candidates = append(candidates, fmt.Sprintf("skill-%s-%s", base, regKey))
		}
		candidates = append(candidates, "skill-"+regKey)
	}
	for _, c := range candidates {
		if canTake(c) {
			return strings.ToLower(c)
		}
	}
	fallback := "skill-" + regKey
	if regKey == "" {
		fallback = "skill-x"
	}
	if len(fallback) > 56 {
		fallback = fallback[:56]
	}
	for n := 0; n < 10_000; n++ {
		t := fallback
		if n > 0 {
			t = fmt.Sprintf("%s-%d", fallback, n)
		}
		if canTake(t) {
			return strings.ToLower(t)
		}
	}
	return strings.ToLower(fallback)
}

func AssignSkillSlashCommands(refs []SkillRefWithKey) []SkillSlashBinding {
	res := ReservedSlashCommandNames()
	claimed := map[string]struct{}{}
	out := make([]SkillSlashBinding, 0, len(refs))
	for _, ref := range refs {
		base := skillSlashBase(ref.Entry.Name)
		chosen := pickSlashName(base, ref.RegistryKey, res, claimed)
		claimed[chosen] = struct{}{}
		out = append(out, SkillSlashBinding{Slash: chosen, Entry: ref.Entry})
	}
	return out
}

func LookupSkillBySlashCommand(slashLower string, projHex, projRoot string) (*SkillEntry, error) {
	slashLower = strings.ToLower(strings.TrimSpace(slashLower))
	if slashLower == "" {
		return nil, nil
	}
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return nil, err
	}
	refs := orderedSkillRefs(reg, projHex, projRoot)
	for _, b := range AssignSkillSlashCommands(refs) {
		if b.Slash == slashLower {
			e := b.Entry
			return &e, nil
		}
	}
	return nil, nil
}
