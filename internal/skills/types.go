package skills

import "encoding/json"

const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
	ScopeLocal   = "local"
)

type SkillEntry struct {
	Name          string         `json:"name"`
	SourceRepo    string         `json:"source_repo"`
	SkillRelPath  string         `json:"skill_rel_path"`
	ClonePath     string         `json:"clone_path"`
	SkillMdPath   string         `json:"skill_md_path"`
	FrontMatter   map[string]any `json:"front_matter,omitempty"`
	AuditSummary  string         `json:"audit_summary,omitempty"`
	SkillSSHPage  string         `json:"skillssh_page,omitempty"`
	InstalledAt   string         `json:"installed_at,omitempty"`
}

type Registry struct {
	Global   map[string]SkillEntry            `json:"global"`
	Projects map[string]map[string]SkillEntry `json:"projects"`
}

func NewRegistry() *Registry {
	return &Registry{
		Global:   map[string]SkillEntry{},
		Projects: map[string]map[string]SkillEntry{},
	}
}

func registryFromJSON(b []byte) (*Registry, error) {
	var r Registry
	if len(b) == 0 {
		return NewRegistry(), nil
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Global == nil {
		r.Global = map[string]SkillEntry{}
	}
	if r.Projects == nil {
		r.Projects = map[string]map[string]SkillEntry{}
	}
	for k, m := range r.Projects {
		if m == nil {
			r.Projects[k] = map[string]SkillEntry{}
		}
	}
	return &r, nil
}

func ApplyScope(r *Registry, scope string, projHex string, skillKey string, e SkillEntry) {
	switch scope {
	case ScopeLocal, ScopeProject:
		if r.Projects[projHex] == nil {
			r.Projects[projHex] = map[string]SkillEntry{}
		}
		r.Projects[projHex][skillKey] = e
	default:
		r.Global[skillKey] = e
	}
}

func ProjectEntries(r *Registry, projHex string) map[string]SkillEntry {
	if r.Projects == nil {
		return map[string]SkillEntry{}
	}
	m := r.Projects[projHex]
	if m == nil {
		return map[string]SkillEntry{}
	}
	out := make(map[string]SkillEntry, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
