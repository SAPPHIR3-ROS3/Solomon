package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

func StableKeyHex(canonicalRepoURL, skillRelPath string) string {
	skillRelPath = strings.TrimSpace(skillRelPath)
	if skillRelPath == "" {
		skillRelPath = "."
	}
	skillRelPath = strings.ReplaceAll(skillRelPath, "\\", "/")
	h := sha256.Sum256([]byte(canonicalRepoURL + "\x00" + skillRelPath))
	return hex.EncodeToString(h[:])
}

func NormalizeRepoURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty repository URL")
	}
	if !strings.Contains(s, "://") && strings.Count(s, "/") == 1 {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			s = "https://github.com/" + parts[0] + "/" + parts[1]
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("only http(s) repositories are supported")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("missing host")
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	u.Fragment = ""
	u.RawQuery = ""
	pathPart := strings.Trim(u.Path, "/")
	pathPart = strings.TrimSuffix(pathPart, ".git")
	if pathPart == "" {
		return "", fmt.Errorf("invalid repository path")
	}
	return "https://" + host + "/" + pathPart, nil
}

func RepoOwner(canonicalRepoURL string) string {
	u, err := url.Parse(strings.TrimSpace(canonicalRepoURL))
	if err != nil {
		return "repo"
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) > 0 && segs[0] != "" {
		return segs[0]
	}
	return "repo"
}
