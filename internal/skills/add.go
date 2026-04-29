package skills

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"solomon/internal/paths"
)

type InstallOpts struct {
	Ctx      context.Context
	Out      io.Writer
	In       io.Reader
	ProjHex  string
	ProjRoot string
	Args     []string
}

type parsedAdd struct {
	SkillsShURL  string
	GitHubRef    string
	DisplayName  string
	Scope        string
	FromSkillsSh bool
}

func isScope(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ScopeGlobal, ScopeProject, ScopeLocal:
		return true
	default:
		return false
	}
}

func ParseAddArgs(parts []string) (*parsedAdd, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("usage: /add https://skills.sh/... [name] [global|project|local] | /add skill <owner/repo|url> [name] [global|project|local]")
	}
	toks := append([]string(nil), parts...)
	scope := ScopeGlobal
	if len(toks) > 0 && isScope(toks[len(toks)-1]) {
		scope = strings.ToLower(strings.TrimSpace(toks[len(toks)-1]))
		toks = toks[:len(toks)-1]
	}
	if len(toks) == 0 {
		return nil, fmt.Errorf("missing arguments after scope")
	}
	p := &parsedAdd{Scope: scope}
	first := strings.TrimSpace(toks[0])
	if strings.HasPrefix(strings.ToLower(first), "https://skills.sh/") {
		p.FromSkillsSh = true
		p.SkillsShURL = first
		switch len(toks) {
		case 1:
			break
		case 2:
			p.DisplayName = strings.TrimSpace(toks[1])
		default:
			return nil, fmt.Errorf("too many arguments for skills.sh URL")
		}
		return p, nil
	}
	if strings.EqualFold(first, "skill") {
		if len(toks) < 2 {
			return nil, fmt.Errorf("usage: /add skill <owner/repo|https://...> [name] [scope]")
		}
		p.GitHubRef = strings.TrimSpace(toks[1])
		if len(toks) == 3 {
			p.DisplayName = strings.TrimSpace(toks[2])
		} else if len(toks) > 3 {
			return nil, fmt.Errorf("too many arguments for skill add")
		}
		return p, nil
	}
	return nil, fmt.Errorf("expected skills.sh URL or 'skill <repo>'")
}

func RunInstall(opts InstallOpts) error {
	p, err := ParseAddArgs(opts.Args)
	if err != nil {
		return err
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var canonical string
	var preferred string
	var pageURL string
	var auditSummary string
	if p.FromSkillsSh {
		meta, err := FetchSkillsShMeta(ctx, p.SkillsShURL)
		if err != nil {
			return err
		}
		pageURL = meta.PageURL
		auditSummary = meta.AuditSummary
		preferred = meta.PreferredSkill
		if p.DisplayName == "" {
			p.DisplayName = meta.DisplayName
		}
		canonical, err = NormalizeRepoURL(meta.RepoURL)
		if err != nil {
			return err
		}
		ok, err := ConfirmInstall(opts.In, opts.Out, meta)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("install cancelled")
		}
	} else {
		canonical, err = NormalizeRepoURL(p.GitHubRef)
		if err != nil {
			return err
		}
	}
	base, err := cloneBaseDir(p.Scope, opts.ProjHex, opts.ProjRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(base, "clone-*")
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()
	cloneURL := canonical
	if !strings.HasSuffix(cloneURL, ".git") {
		cloneURL = cloneURL + ".git"
	}
	if err := CloneOrPull(ctx, cloneURL, tempDir); err != nil {
		return err
	}
	skillRel, _, err := LocateSkillDir(tempDir, preferred)
	if err != nil {
		return err
	}
	skillKey := StableKeyHex(canonical, skillRel)
	finalDir := filepath.Join(base, skillKey)
	_ = os.RemoveAll(finalDir)
	if err := os.Rename(tempDir, finalDir); err != nil {
		return fmt.Errorf("move clone into place: %w", err)
	}
	removeTemp = false
	cloneAbs, err := filepath.Abs(finalDir)
	if err != nil {
		return err
	}
	mdRel := filepath.Join(filepath.FromSlash(skillRel), "SKILL.md")
	mdAbs := filepath.Join(cloneAbs, mdRel)
	if _, err := os.Stat(mdAbs); err != nil {
		mdAbs = filepath.Join(cloneAbs, "SKILL.md")
	}
	mdAbs, err = filepath.Abs(mdAbs)
	if err != nil {
		return err
	}
	fm, err := ParseSkillFrontMatter(mdAbs)
	if err != nil {
		return err
	}
	display := p.DisplayName
	if strings.TrimSpace(display) == "" {
		display = DisplayNameFromFrontMatter(fm, filepath.Base(filepath.Dir(mdAbs)))
	}
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return err
	}
	lockPath, err := paths.SkillsRegistryLockPath()
	if err != nil {
		return err
	}
	return WithRegistryLock(lockPath, regPath, func(r *Registry) error {
		final := UniqueDisplayName(r, canonical, display, p.Scope, opts.ProjHex, skillKey)
		if final != strings.TrimSpace(display) && opts.Out != nil {
			fmt.Fprintf(opts.Out, "Display name %q already in use; using %q.\n", strings.TrimSpace(display), final)
		}
		entry := SkillEntry{
			Name:         final,
			SourceRepo:   canonical,
			SkillRelPath: skillRel,
			ClonePath:    cloneAbs,
			SkillMdPath:  mdAbs,
			FrontMatter:  fm,
			AuditSummary: auditSummary,
			SkillSSHPage: pageURL,
			InstalledAt:  time.Now().UTC().Format(time.RFC3339),
		}
		ApplyScope(r, p.Scope, opts.ProjHex, skillKey, entry)
		if p.Scope == ScopeLocal {
			mirror := paths.LocalSkillsMirrorPath(opts.ProjRoot)
			return SaveMirrorJSON(mirror, ProjectEntries(r, opts.ProjHex))
		}
		return nil
	})
}

func cloneBaseDir(scope, projHex, projRoot string) (string, error) {
	switch scope {
	case ScopeGlobal:
		return paths.GlobalSkillsDir()
	case ScopeProject:
		return paths.ProjectSkillsDir(projHex)
	case ScopeLocal:
		return paths.LocalSkillsDir(projRoot), nil
	default:
		return "", fmt.Errorf("invalid scope %q", scope)
	}
}
