package skills

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SkillsShMeta struct {
	PageURL        string
	RepoURL        string
	PreferredSkill string
	DisplayName    string
	AuditSummary   string
}

var reNpxSkills = regexp.MustCompile(`npx\s+skills\s+add\s+([^\s<\\]+)(?:\s+\\?\s*--skill\s+([^\s<\\\n]+))?`)
var reGitHubHTTPS = regexp.MustCompile(`https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+`)
var reTotalInstalls = regexp.MustCompile(`([\d.]+\s*[KMB]?)\s+total\s+installs`)

func IsSkillsShURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Host, "."))
	return host == "skills.sh" || host == "www.skills.sh"
}

func NormalizeSkillsShURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("skills.sh URL must be https://skills.sh/...")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Host, "."))
	if host != "skills.sh" && host != "www.skills.sh" {
		return "", fmt.Errorf("skills.sh URL must be https://skills.sh/...")
	}
	u.Host = "skills.sh"
	u.Fragment = ""
	return u.String(), nil
}

func FetchSkillsShMeta(ctx context.Context, pageURL string) (*SkillsShMeta, error) {
	pageURL, err := NormalizeSkillsShURL(pageURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 45 * time.Second}
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
		return nil, fmt.Errorf("skills.sh: HTTP %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return parseSkillsShFromHTML(string(body), pageURL)
}

func parseSkillsShFromHTML(html, pageURL string) (*SkillsShMeta, error) {
	u, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil {
		return nil, err
	}
	meta := &SkillsShMeta{PageURL: pageURL}
	if m := reNpxSkills.FindStringSubmatch(html); len(m) >= 2 {
		repoRef := strings.TrimSpace(m[1])
		if strings.HasPrefix(repoRef, "https://") || strings.HasPrefix(repoRef, "http://") {
			meta.RepoURL = strings.TrimSuffix(repoRef, "\\")
		} else {
			meta.RepoURL = "https://github.com/" + strings.TrimSuffix(repoRef, "\\")
		}
		if len(m) >= 3 && strings.TrimSpace(m[2]) != "" {
			meta.PreferredSkill = strings.Trim(strings.TrimSpace(strings.TrimSuffix(m[2], "\\")), `"`)
		}
	}
	if meta.RepoURL == "" {
		if g := reGitHubHTTPS.FindString(html); g != "" {
			meta.RepoURL = strings.TrimSuffix(g, ".git")
		}
	}
	if meta.RepoURL == "" {
		return nil, fmt.Errorf("could not find repository URL on skills.sh page (try: /add npx skills add https://github.com/owner/repo --skill <pkg>)")
	}
	if meta.DisplayName == "" {
		p := strings.Trim(u.Path, "/")
		if p != "" {
			parts := strings.Split(p, "/")
			meta.DisplayName = parts[len(parts)-1]
		}
	}
	if m := reTotalInstalls.FindStringSubmatch(html); len(m) >= 2 {
		meta.AuditSummary = "Reported installs: " + strings.TrimSpace(m[1]) + " total (from skills.sh page)"
	}
	return meta, nil
}

func (m *SkillsShMeta) InstallShellCommand() string {
	repo := strings.TrimSuffix(strings.TrimSpace(m.RepoURL), "/")
	sk := strings.TrimSpace(m.PreferredSkill)
	if sk != "" {
		return fmt.Sprintf("npx --yes skills add %s --skill %s -y", repo, sk)
	}
	return fmt.Sprintf("npx --yes skills add %s -y", repo)
}

func ConfirmInstall(in io.Reader, out io.Writer, meta *SkillsShMeta) (bool, error) {
	fmt.Fprintf(out, "Repository: %s\n", meta.RepoURL)
	if meta.PreferredSkill != "" {
		fmt.Fprintf(out, "Skill package: %s\n", meta.PreferredSkill)
	}
	if meta.AuditSummary != "" {
		fmt.Fprintf(out, "%s\n", meta.AuditSummary)
	}
	fmt.Fprint(out, "Install this skill? [y/N]: ")
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}
