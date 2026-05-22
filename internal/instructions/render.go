package instructions

import (
	"fmt"
	"strings"
)

type PromptSections struct {
	CustomRules        string
	GlobalInstructions string
	RepoInstructions   string
}

func (l *Loader) BuildPromptSections(projRoot, projHex string, activatedDirs []string) (PromptSections, error) {
	var out PromptSections
	rules, err := LoadAllRulesText(projHex)
	if err != nil {
		return out, err
	}
	out.CustomRules = strings.TrimSpace(rules)

	if _, content, ok := l.LoadGlobal(); ok {
		out.GlobalInstructions = strings.TrimSpace(content)
	}

	var repo strings.Builder
	if _, rootContent, ok := l.LoadRepoRoot(projRoot); ok {
		repo.WriteString("### /\n\n")
		repo.WriteString(strings.TrimSpace(rootContent))
		repo.WriteByte('\n')
	}
	for _, rel := range activatedDirs {
		rel = strings.TrimSpace(filepathToSlash(rel))
		if rel == "" || rel == "." || rel == "/" {
			continue
		}
		if _, content, ok := l.LoadRepoDir(projRoot, rel); ok {
			if repo.Len() > 0 {
				repo.WriteByte('\n')
			}
			fmt.Fprintf(&repo, "### %s/\n\n", rel)
			repo.WriteString(strings.TrimSpace(content))
			repo.WriteByte('\n')
		}
	}
	out.RepoInstructions = strings.TrimSpace(repo.String())
	return out, nil
}

func filepathToSlash(s string) string {
	return strings.ReplaceAll(s, `\`, `/`)
}

func FormatInstructionBlock(sections PromptSections) string {
	var parts []string
	if s := strings.TrimSpace(sections.CustomRules); s != "" {
		parts = append(parts, "## Custom rules\n\n"+s)
	}
	if s := strings.TrimSpace(sections.GlobalInstructions); s != "" {
		parts = append(parts, "## Global instructions\n\n"+s)
	}
	if s := strings.TrimSpace(sections.RepoInstructions); s != "" {
		parts = append(parts, "## Repository instructions\n\n"+s)
	}
	return strings.Join(parts, "\n\n")
}
