// Package desktopgit parses the machine-readable Git output used by the
// desktop bridge.
package desktopgit

import "strings"

// Commit is one entry from git log's NUL-delimited desktop history format.
type Commit struct {
	Author     string   `json:"author"`
	AuthoredAt string   `json:"authoredAt"`
	Hash       string   `json:"hash"`
	Parents    []string `json:"parents"`
	Refs       []string `json:"refs"`
	ShortHash  string   `json:"shortHash"`
	Subject    string   `json:"subject"`
}

// ParseHistory parses the NUL-delimited records produced by the desktop Git
// history command.
func ParseHistory(output []byte) []Commit {
	commits := make([]Commit, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\x00")
		if len(fields) < 7 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		shortHash := strings.TrimSpace(fields[1])
		subject := strings.TrimSpace(fields[4])
		if hash == "" || shortHash == "" || subject == "" {
			continue
		}
		commits = append(commits, Commit{
			Author:     strings.TrimSpace(fields[2]),
			AuthoredAt: strings.TrimSpace(fields[3]),
			Hash:       hash,
			Parents:    strings.Fields(fields[6]),
			Refs:       splitRefs(fields[5]),
			ShortHash:  shortHash,
			Subject:    subject,
		})
	}
	return commits
}

// ParseStatus parses the NUL-delimited records produced by git diff
// --name-status and retains the destination path for renames and copies.
func ParseStatus(output []byte) map[string]string {
	status := make(map[string]string)
	fields := strings.Split(string(output), "\x00")
	for index := 0; index < len(fields); {
		code := strings.TrimSpace(fields[index])
		index++
		if code == "" {
			continue
		}
		statusCode := code[0]
		if statusCode == 'R' || statusCode == 'C' {
			if index < len(fields) {
				index++
			}
			if index < len(fields) {
				if filePath := normalizePath(fields[index]); filePath != "" {
					status[filePath] = string(statusCode)
				}
				index++
			}
			continue
		}
		if index >= len(fields) {
			break
		}
		if filePath := normalizePath(fields[index]); filePath != "" {
			status[filePath] = string(statusCode)
		}
		index++
	}
	return status
}

func normalizePath(value string) string {
	return strings.TrimSuffix(value, "\r")
}

func splitRefs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	refs := make([]string, 0, 2)
	for _, value := range strings.Split(value, ",") {
		if ref := strings.TrimSpace(value); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}
