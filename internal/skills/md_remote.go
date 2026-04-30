package skills

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"solomon/internal/logging"
	"solomon/internal/paths"
)

func NormalizeMarkdownSourceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("only http(s) URLs are supported for remote SKILL.md")
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	u.Fragment = ""
	return u.String(), nil
}

func IsRemoteMarkdownURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	path := strings.ToLower(u.Path)
	return strings.HasSuffix(path, ".md")
}

func IsSkillMarkdownSource(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	if IsRemoteMarkdownURL(s) {
		return true
	}
	lower := strings.ToLower(s)
	if strings.Contains(s, "://") {
		if strings.HasPrefix(lower, "file://") {
			return strings.HasSuffix(lower, ".md")
		}
		return false
	}
	return strings.HasSuffix(lower, ".md")
}

func pathFromFileURL(u *url.URL) (string, error) {
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file URL")
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.FromSlash(p), nil
}

func NormalizeSkillMarkdownSource(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if IsRemoteMarkdownURL(raw) {
		return NormalizeMarkdownSourceURL(raw)
	}
	p := raw
	if strings.HasPrefix(strings.ToLower(p), "file://") {
		u, err := url.Parse(p)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(u.Scheme, "file") {
			return "", fmt.Errorf("invalid file URL")
		}
		var err2 error
		p, err2 = pathFromFileURL(u)
		if err2 != nil {
			return "", err2
		}
	} else if strings.Contains(p, "://") && !strings.HasPrefix(strings.ToLower(p), "file://") {
		return "", fmt.Errorf("unsupported URL scheme for skill markdown (use https URL ending in .md, file:// path, or a local path)")
	}
	if !strings.HasSuffix(strings.ToLower(p), ".md") {
		return "", fmt.Errorf("skill source must be a .md file")
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("skill markdown file: %w", err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("skill source is a directory")
	}
	return abs, nil
}

func downloadMarkdown(ctx context.Context, pageURL string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Solomon/1.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s fetching %s", res.Status, pageURL)
	}
	return io.ReadAll(io.LimitReader(res.Body, 5<<20))
}

func readLocalMarkdownFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 5<<20))
}

func EnrichFrontMatterInteractive(in io.Reader, out io.Writer, fm map[string]any, displayOverride string) error {
	if fm == nil {
		return fmt.Errorf("internal: nil front matter map")
	}
	if strings.TrimSpace(displayOverride) != "" {
		_, hasName := fm["name"]
		_, hasTitle := fm["title"]
		if !hasName && !hasTitle {
			fm["name"] = strings.TrimSpace(displayOverride)
		}
	}
	nameStr := ""
	for _, k := range []string{"name", "title"} {
		if v, ok := fm[k]; ok {
			if s, ok := v.(string); ok {
				nameStr = strings.TrimSpace(s)
				if nameStr != "" {
					break
				}
			}
		}
	}
	br := bufio.NewReader(in)
	if nameStr == "" {
		if out != nil {
			fmt.Fprint(out, "Skill name (required): ")
		}
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		nameStr = strings.TrimSpace(line)
		if nameStr == "" {
			return fmt.Errorf("skill name cannot be empty")
		}
		fm["name"] = nameStr
	}
	desc := ""
	if v, ok := fm["description"]; ok {
		if s, ok := v.(string); ok {
			desc = strings.TrimSpace(s)
		}
	}
	if desc == "" {
		if out != nil {
			fmt.Fprint(out, "Description (optional, Enter to skip): ")
		}
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if d := strings.TrimSpace(line); d != "" {
			fm["description"] = d
		}
	}
	return nil
}

func runRemoteMDInstall(opts InstallOpts, p *parsedAdd) error {
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := NormalizeSkillMarkdownSource(p.RemoteMDURL)
	if err != nil {
		return err
	}
	var raw []byte
	if strings.HasPrefix(strings.ToLower(normalized), "https://") {
		raw, err = downloadMarkdown(ctx, normalized)
		if err != nil {
			return fmt.Errorf("download skill markdown: %w", err)
		}
	} else {
		raw, err = readLocalMarkdownFile(normalized)
		if err != nil {
			return fmt.Errorf("read skill markdown: %w", err)
		}
	}
	fm, body, err := ParseSkillBytes(raw)
	if err != nil {
		return err
	}
	if err := EnrichFrontMatterInteractive(opts.In, opts.Out, fm, p.DisplayName); err != nil {
		return err
	}
	skillKey := StableKeyHex(normalized, ".")
	base, err := cloneBaseDir(p.Scope, opts.ProjHex, opts.ProjRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	finalDir := filepath.Join(base, skillKey)
	if err := os.RemoveAll(finalDir); err != nil {
		return err
	}
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return err
	}
	mdAbs := filepath.Join(finalDir, "SKILL.md")
	if err := WriteSkillMarkdown(mdAbs, fm, body); err != nil {
		return err
	}
	cloneAbs, err := filepath.Abs(finalDir)
	if err != nil {
		return err
	}
	mdAbs, err = filepath.Abs(mdAbs)
	if err != nil {
		return err
	}
	fmFinal, err := ParseSkillFrontMatter(mdAbs)
	if err != nil {
		return err
	}
	display := p.DisplayName
	if strings.TrimSpace(display) == "" {
		display = DisplayNameFromFrontMatter(fmFinal, strings.TrimSuffix(filepath.Base(mdAbs), ".md"))
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
		final := UniqueDisplayName(r, normalized, display, p.Scope, opts.ProjHex, skillKey)
		if final != strings.TrimSpace(display) && opts.Out != nil {
			fmt.Fprintf(opts.Out, "Display name %q already in use; using %q.\n", strings.TrimSpace(display), final)
		}
		entry := SkillEntry{
			Name:         final,
			SourceRepo:   normalized,
			SkillRelPath: ".",
			ClonePath:    cloneAbs,
			SkillMdPath:  mdAbs,
			FrontMatter:  fmFinal,
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
	logging.Log(logging.INFO_LOG_LEVEL, "skill remote md install complete", logging.LogOptions{Params: map[string]any{"scope": p.Scope, "url": normalized}})
	if opts.Out != nil {
		fmt.Fprintf(opts.Out, "Skill installed from %s.\n", normalized)
	}
	return nil
}
