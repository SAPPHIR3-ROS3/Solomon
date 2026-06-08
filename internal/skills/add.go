package skills

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
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
	FromSkillsSh bool
	SkillsShURL  string
	FromRemoteMD bool
	RemoteMDURL  string
	NpmCommand   string
	DisplayName  string
	Scope        string
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
		return nil, fmt.Errorf(`usage: /add npx ... | skills.sh | skill <.md> [name] [scope]`)
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
	low := strings.ToLower(first)
	if low == "npx" || low == "npm" {
		p.NpmCommand = strings.Join(toks, " ")
		return p, nil
	}
	if IsSkillsShURL(first) {
		pageURL, err := NormalizeSkillsShURL(first)
		if err != nil {
			return nil, err
		}
		p.FromSkillsSh = true
		p.SkillsShURL = pageURL
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
			return nil, fmt.Errorf(`usage: /add skill <.md path|URL> [name] [scope]`)
		}
		u := strings.TrimSpace(toks[1])
		if IsSkillsShURL(u) {
			pageURL, err := NormalizeSkillsShURL(u)
			if err != nil {
				return nil, err
			}
			p.FromSkillsSh = true
			p.SkillsShURL = pageURL
			switch len(toks) {
			case 2:
			case 3:
				p.DisplayName = strings.TrimSpace(toks[2])
			default:
				return nil, fmt.Errorf("too many arguments for skills.sh URL")
			}
			return p, nil
		}
		if !IsSkillMarkdownSource(u) {
			return nil, fmt.Errorf("/add skill: .md via https, file://, or local path (for skills.sh use: /add https://skills.sh/...)")
		}
		p.FromRemoteMD = true
		p.RemoteMDURL = u
		switch len(toks) {
		case 2:
		case 3:
			p.DisplayName = strings.TrimSpace(toks[2])
		default:
			return nil, fmt.Errorf("too many arguments for /add skill")
		}
		return p, nil
	}
	return nil, fmt.Errorf(`expected npx/npm line, skills.sh URL, or skill <.md>`)
}

func RunInstall(opts InstallOpts) (err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "skill install failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}()
	p, err := ParseAddArgs(opts.Args)
	if err != nil {
		return err
	}
	if p.FromRemoteMD {
		return runRemoteMDInstall(opts, p)
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var meta *SkillsShMeta
	var pageURL, auditSummary string
	if p.FromSkillsSh {
		meta, err = FetchSkillsShMeta(ctx, p.SkillsShURL)
		if err != nil {
			return err
		}
		pageURL = meta.PageURL
		auditSummary = meta.AuditSummary
		if p.DisplayName == "" {
			p.DisplayName = meta.DisplayName
		}
		shCmd := EnsureSkillsAddGlobalYes(meta.InstallShellCommand())
		termcolor.WriteSystem(opts.Out, fmt.Sprintf("Command: %s", shCmd))
		ok, err := ConfirmInstall(opts.In, opts.Out, meta)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("install cancelled")
		}
		p.NpmCommand = shCmd
	}
	if strings.TrimSpace(p.NpmCommand) == "" {
		return fmt.Errorf("no install command (internal error)")
	}
	if err := RequireNpm(ctx); err != nil {
		return err
	}
	agentsRoot, err := AgentsSkillsRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(agentsRoot, 0o700); err != nil {
		return err
	}
	before, err := snapAgentsSkills(agentsRoot)
	if err != nil {
		return err
	}
	installCmd := EnsureSkillsAddGlobalYes(strings.TrimSpace(p.NpmCommand))
	validatedCmd, err := parseSkillsInstallCommand(installCmd)
	if err != nil {
		return err
	}
	installCmd = validatedCmd.Display
	logging.Log(logging.INFO_LOG_LEVEL, "skill npm install", logging.LogOptions{Params: map[string]any{"command": installCmd}})
	if err := runInstallShellCommand(ctx, installCmd, opts.Out, opts.Out); err != nil {
		return fmt.Errorf("npm/skills install failed: %w", err)
	}
	after, err := snapAgentsSkills(agentsRoot)
	if err != nil {
		return err
	}
	preferred := ""
	if meta != nil {
		preferred = meta.PreferredSkill
	}
	picked, err := pickImportedSkillDir(before, after, preferred)
	if err != nil {
		return err
	}
	srcDir := filepath.Join(agentsRoot, picked)
	canonical, err := canonicalForRegistry(installCmd, meta)
	if err != nil {
		return err
	}
	skillRel := strings.ReplaceAll(picked, "\\", "/")
	skillKey := StableKeyHex(canonical, skillRel)
	base, err := cloneBaseDir(p.Scope, opts.ProjHex, opts.ProjRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	finalDir := filepath.Join(base, skillKey)
	if err := copySkillTree(srcDir, finalDir); err != nil {
		return fmt.Errorf("copy from ~/.agents/skills: %w", err)
	}
	cloneAbs, err := filepath.Abs(finalDir)
	if err != nil {
		return err
	}
	skillRelDir, mdAbs, err := LocateSkillDir(cloneAbs, "")
	if err != nil {
		return err
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
	if err := WithRegistryLock(lockPath, regPath, func(r *Registry) error {
		final := UniqueDisplayName(r, canonical, display, p.Scope, opts.ProjHex, skillKey)
		if final != strings.TrimSpace(display) && opts.Out != nil {
			termcolor.WriteSystem(opts.Out, fmt.Sprintf("Display name %q already in use; using %q.", strings.TrimSpace(display), final))
		}
		entry := SkillEntry{
			Name:         final,
			SourceRepo:   canonical,
			SkillRelPath: skillRelDir,
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
	}); err != nil {
		return err
	}
	logging.Log(logging.INFO_LOG_LEVEL, "skill install complete", logging.LogOptions{Params: map[string]any{"scope": p.Scope, "repo": canonical, "folder": picked}})
	if opts.Out != nil {
		termcolor.WriteSystem(opts.Out, fmt.Sprintf("Skill copied from ~/.agents/skills/%s into Solomon registry.", picked))
	}
	return nil
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
