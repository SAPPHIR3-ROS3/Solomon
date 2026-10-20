package logging

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ReportByteLimit = 32 * 1024
const ReportNoteLimit = 500

var reportEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
var reportIPv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var reportBearer = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
var reportPrefixedSecret = regexp.MustCompile(`\b(?:sk|pk|api|key|token)[-_][A-Za-z0-9_\-]{8,}\b`)
var reportLongSecret = regexp.MustCompile(`\b[A-Za-z0-9+/_-]{40,}={0,2}\b`)
var reportPath = regexp.MustCompile(`(?:[A-Za-z]:\\|/|~/)(?:[\w.+@~-]+[/\\])+[\w.+@~-]*`)

func RedactReport(text string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		text = strings.ReplaceAll(text, home, "<path>")
	}
	if current, err := user.Current(); err == nil {
		text = replaceIdent(text, current.Username)
		text = replaceIdent(text, current.Name)
	}
	text = replaceIdent(text, os.Getenv("USER"))
	text = replaceIdent(text, os.Getenv("USERNAME"))
	text = reportEmail.ReplaceAllString(text, "<email>")
	text = reportBearer.ReplaceAllString(text, "Bearer <secret>")
	text = reportPrefixedSecret.ReplaceAllString(text, "<secret>")
	text = reportPath.ReplaceAllString(text, "<path>")
	text = reportLongSecret.ReplaceAllString(text, "<secret>")
	text = reportIPv4.ReplaceAllString(text, "<ip>")
	return text
}

func replaceIdent(text, ident string) string {
	ident = strings.TrimSpace(ident)
	if len(ident) < 3 {
		return text
	}
	return regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(ident)+`\b`).ReplaceAllString(text, "<user>")
}

func ReportLogs() (string, error) {
	logMu.Lock()
	dir := globalLogDir
	logMu.Unlock()
	if dir == "" {
		return "", nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	var newestFirst []string
	raw := 0
	for _, path := range files {
		lines, err := reportLines(path)
		if err != nil {
			return "", err
		}
		for i := len(lines) - 1; i >= 0; i-- {
			line := lines[i]
			if !reportWanted(line) {
				continue
			}
			newestFirst = append(newestFirst, line)
			raw += len(line) + 1
			if raw >= ReportByteLimit*8 {
				break
			}
		}
		if raw >= ReportByteLimit*8 {
			break
		}
	}
	for left, right := 0, len(newestFirst)-1; left < right; left, right = left+1, right-1 {
		newestFirst[left], newestFirst[right] = newestFirst[right], newestFirst[left]
	}
	redacted := make([]string, len(newestFirst))
	for i, line := range newestFirst {
		redacted[i] = RedactReport(line)
	}
	for len(redacted) > 0 && reportSize(redacted) > ReportByteLimit {
		redacted = redacted[1:]
	}
	if len(redacted) == 1 && len(redacted[0]) > ReportByteLimit {
		redacted[0] = redacted[0][len(redacted[0])-ReportByteLimit:]
	}
	return strings.Join(redacted, "\n"), nil
}

func reportSize(lines []string) int {
	size := 0
	for _, line := range lines {
		size += len(line) + 1
	}
	if size > 0 {
		size--
	}
	return size
}

func reportWanted(line string) bool {
	return strings.Contains(line, "][ERROR][") || strings.Contains(line, "][WARNING][") || strings.Contains(line, "][INFO][")
}

func reportLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func ReportStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
