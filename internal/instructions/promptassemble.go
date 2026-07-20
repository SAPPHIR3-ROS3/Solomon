package instructions

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
)

type PromptSections struct {
	CustomRules        string
	GlobalInstructions string
	RepoInstructions   string
}

func (l *Loader) BuildPromptSections(ctx context.Context, projRoot, projHex string, activatedDirs []string, notify *atmention.Notifier) (PromptSections, error) {
	var out PromptSections
	rules, err := LoadAllRulesText(projHex)
	if err != nil {
		return out, err
	}
	out.CustomRules = strings.TrimSpace(rules)

	if path, content, ok := l.LoadGlobal(); ok {
		out.GlobalInstructions = strings.TrimSpace(l.expandAtIncludes(ctx, content, path, projRoot, notify))
	}

	var repo strings.Builder
	if path, rootContent, ok := l.LoadRepoRoot(projRoot); ok {
		repo.WriteString("### /\n\n")
		repo.WriteString(strings.TrimSpace(l.expandAtIncludes(ctx, rootContent, path, projRoot, notify)))
		repo.WriteByte('\n')
	}
	for _, rel := range activatedDirs {
		rel = strings.TrimSpace(filepathToSlash(rel))
		if rel == "" || rel == "." || rel == "/" {
			continue
		}
		if path, content, ok := l.LoadRepoDir(projRoot, rel); ok {
			if repo.Len() > 0 {
				repo.WriteByte('\n')
			}
			fmt.Fprintf(&repo, "### %s/\n\n", rel)
			repo.WriteString(strings.TrimSpace(l.expandAtIncludes(ctx, content, path, projRoot, notify)))
			repo.WriteByte('\n')
		}
	}
	out.RepoInstructions = strings.TrimSpace(repo.String())
	return out, nil
}

func (l *Loader) expandAtIncludes(ctx context.Context, raw, sourcePath, projRoot string, notify *atmention.Notifier) string {
	if !strings.Contains(raw, "@") {
		return raw
	}
	expanded, err := atmention.ExpandDocumentWithMax(ctx, raw, sourcePath, projRoot, l.maxBytes(), notify)
	if err != nil {
		return raw
	}
	return expanded
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
