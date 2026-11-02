package clone

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Identity struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func gitCredential(remote string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "credential", "fill")
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false")
	command.Stdin = strings.NewReader("url=" + remote + "\n\n")
	output, err := command.Output()
	if err != nil {
		return "", ""
	}
	var username, password string
	for _, line := range strings.Split(string(output), "\n") {
		key, value, _ := strings.Cut(line, "=")
		if key == "username" {
			username = value
		}
		if key == "password" {
			password = value
		}
	}
	return username, password
}

func identities(remote string) []Identity {
	result := []Identity{}
	remote = normalizeRemote(remote)
	parsed, err := url.Parse(remote)
	if err != nil {
		return result
	}
	if parsed.Scheme == "https" || parsed.Host == "" {
		username, password := "", ""
		if parsed.Host != "" {
			username, password = gitCredential(remote)
		}
		if username != "" && password != "" {
			result = append(result, Identity{"helper", username + " · " + parsed.Hostname() + " (Git credentials)"})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		output, _ := exec.CommandContext(ctx, "gh", "auth", "status", "--json", "hosts").Output()
		var status struct {
			Hosts map[string][]struct {
				Login string `json:"login"`
			} `json:"hosts"`
		}
		if json.Unmarshal(output, &status) == nil {
			for host, accounts := range status.Hosts {
				if parsed.Host != "" && parsed.Hostname() != host {
					continue
				}
				if parsed.Host == "" {
					username, password := gitCredential("https://" + host + "/")
					if username != "" && password != "" {
						result = append(result, Identity{"helper:" + host, username + " · " + host + " (Git credentials)"})
					}
				}
				for _, account := range accounts {
					if account.Login != "" {
						result = append(result, Identity{"gh:" + host + ":" + account.Login, account.Login + " · " + host + " (GitHub)"})
					}
				}
			}
		}
	}
	{
		home, _ := os.UserHomeDir()
		entries, _ := os.ReadDir(filepath.Join(home, ".ssh"))
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".pub") {
				name := strings.TrimSuffix(entry.Name(), ".pub")
				if info, err := os.Stat(filepath.Join(home, ".ssh", name)); err == nil && !info.IsDir() {
					data, _ := os.ReadFile(filepath.Join(home, ".ssh", entry.Name()))
					fields := strings.Fields(string(data))
					label := name
					if len(fields) > 2 {
						label = fields[2] + " · " + name
					}
					result = append(result, Identity{name, label + " (SSH key)"})
				}
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		priority := func(identity Identity) int {
			if strings.HasSuffix(identity.Label, " (SSH key)") {
				return 2
			}
			if identity.ID == "helper" || strings.HasPrefix(identity.ID, "helper:") {
				return 1
			}
			return 0
		}
		left, right := priority(result[i]), priority(result[j])
		if left != right {
			return left < right
		}
		return result[i].Label < result[j].Label
	})
	return result
}

func normalizeRemote(remote string) string {
	if !strings.Contains(remote, "://") {
		if userHost, path, ok := strings.Cut(remote, ":"); ok && strings.Contains(userHost, "@") {
			return "ssh://" + userHost + "/" + path
		}
	}
	return remote
}
func hasPassword(remote *url.URL) bool { _, exists := remote.User.Password(); return exists }
