package skills

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func InstalledSkillCount(projHex, projRoot string) (int, error) {
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return 0, err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return 0, err
	}
	locals, projects, globals := PartitionInstalledSkills(reg, projHex, projRoot)
	return len(locals) + len(projects) + len(globals), nil
}

func ListInstalledSkills(w io.Writer, projHex, projRoot string) error {
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return err
	}
	return WriteInstalledSkillsSections(w, reg, projHex, projRoot)
}

func WriteInstalledSkillsSections(w io.Writer, r *Registry, projHex, projRoot string) error {
	locals, projects, globals := PartitionInstalledSkills(r, projHex, projRoot)
	sortSkillEntries(locals)
	sortSkillEntries(projects)
	sortSkillEntries(globals)
	any := len(locals)+len(projects)+len(globals) > 0
	writeSkillSection(w, "Local", locals)
	writeSkillSection(w, "Project", projects)
	writeSkillSection(w, "Global", globals)
	if !any {
		fmt.Fprintln(w, "No skills installed.")
	}
	return nil
}

func PartitionInstalledSkills(r *Registry, projHex, projRoot string) (local, project, global []SkillEntry) {
	for _, e := range r.Global {
		global = append(global, e)
	}
	if projHex == "" {
		return local, project, global
	}
	m := r.Projects[projHex]
	if m == nil {
		return local, project, global
	}
	for _, e := range m {
		if projRoot != "" && clonePathUnderLocal(e, projRoot) {
			local = append(local, e)
			continue
		}
		if clonePathUnderProject(e, projHex) {
			project = append(project, e)
			continue
		}
		project = append(project, e)
	}
	return local, project, global
}

func clonePathUnderLocal(e SkillEntry, projRoot string) bool {
	p := strings.TrimSpace(e.ClonePath)
	if p == "" {
		return false
	}
	return relUnderResolved(p, paths.LocalSkillsDir(projRoot))
}

func clonePathUnderProject(e SkillEntry, projHex string) bool {
	p := strings.TrimSpace(e.ClonePath)
	if p == "" {
		return false
	}
	dir, err := paths.ProjectSkillsDir(projHex)
	if err != nil {
		return false
	}
	return relUnderResolved(p, dir)
}

func sortSkillEntries(s []SkillEntry) {
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(strings.TrimSpace(s[i].Name)) < strings.ToLower(strings.TrimSpace(s[j].Name))
	})
}

func writeSkillSection(w io.Writer, title string, entries []SkillEntry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(w, "%s\n", title)
	for _, e := range entries {
		line := strings.TrimSpace(e.Name)
		src := strings.TrimSpace(e.SourceRepo)
		if src == "" {
			src = strings.TrimSpace(e.SkillMdPath)
		}
		if src != "" {
			fmt.Fprintf(w, "  • %s — %s\n", line, src)
		} else {
			fmt.Fprintf(w, "  • %s\n", line)
		}
	}
	fmt.Fprintln(w)
}

func SkillHelpCommand(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "/skill"
	}
	var b strings.Builder
	b.WriteByte('/')
	prevDash := false
	for _, r := range name {
		switch {
		case r == ' ' || r == '\t' || r == '\n':
			if b.Len() > 1 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		case r == '/' || r == '\\':
			continue
		default:
			b.WriteRune(r)
			prevDash = false
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if s == "/" {
		return "/skill"
	}
	return s
}

func WriteSkillInstallHelpSection(w io.Writer, cmdColMin int) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Skill install")
	type row struct {
		cmd, detail string
	}
	rows := []row{
		{"/add", "skills.sh URL | npx skills add ... | skill <.md> [name] [scope]"},
		{"global", "default — ~/.solomon/skills/ (all projects)"},
		{"project", "~/.solomon/projects/<id>/skills/ (registered cwd tree)"},
		{"local", "<workspace>/.solomon/skills/ (this repo only)"},
		{"skills.sh", "https://skills.sh/owner/repo/pkg (www. accepted; no extra skill prefix)"},
		{"example", "/add https://skills.sh/anthropics/skills/prd project"},
	}
	maxW := cmdColMin
	for _, r := range rows {
		if n := len(r.cmd); n > maxW {
			maxW = n
		}
	}
	for _, r := range rows {
		fmt.Fprintf(w, "%-*s\t%s\n", maxW, r.cmd, r.detail)
	}
}

func WriteSkillsHelpSection(w io.Writer, cmdColMin int, projHex, projRoot string) error {
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return err
	}
	binds := AssignSkillSlashCommands(OrderedSkillRefs(reg, projHex, projRoot))
	if len(binds) == 0 {
		return nil
	}
	maxW := cmdColMin
	for _, b := range binds {
		if n := len("/" + b.Slash); n > maxW {
			maxW = n
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Skills")
	for _, b := range binds {
		cmd := "/" + b.Slash
		desc := DescriptionFromFrontMatter(b.Entry.FrontMatter)
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%-*s\t%s\n", maxW, cmd, desc)
	}
	return nil
}
