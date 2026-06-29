package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

var exportMultiDashRE = regexp.MustCompile(`-+`)
var exportSuffixFileRE = regexp.MustCompile(`^(.+)-(\d+)\.md$`)

const exportTitleSlugMaxRunes = 80

func exportChatBasename(sess *chatstore.Session) string {
	if sess == nil {
		return "untitled"
	}
	title := strings.TrimSpace(sess.Title)
	if title != "" {
		return exportSlugTitle(title)
	}
	id := strings.TrimSpace(sess.ID)
	if id != "" {
		return exportSlugTitle(id)
	}
	ts := sess.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return exportSlugTitle(chatstore.NewPlaceholderChatID(ts))
}

func exportSlugTitle(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "untitled"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(exportMultiDashRE.ReplaceAllString(b.String(), "-"), "-")
	if out == "" {
		out = "untitled"
	}
	if r := []rune(out); len(r) > exportTitleSlugMaxRunes {
		out = strings.Trim(string(r[:exportTitleSlugMaxRunes]), "-")
	}
	if out == "" {
		return "untitled"
	}
	return out
}

type exportPathPlan struct {
	AbsolutePath string
	DateDir      string
	Basename     string
}

func planExportPath(rootDir string, day time.Time, base string, rejectIfExists bool) (exportPathPlan, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return exportPathPlan{}, fmt.Errorf("export root path is empty")
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return exportPathPlan{}, fmt.Errorf("export basename is empty")
	}
	dateDir := filepath.Join(rootDir, day.UTC().Format("2006-01-02"))
	hasBase, maxSuffix, err := exportExistingMatches(dateDir, base)
	if err != nil {
		return exportPathPlan{}, err
	}
	if rejectIfExists && (hasBase || maxSuffix >= 0) {
		return exportPathPlan{}, fmt.Errorf("chat already exported (matching %s*.md in %s)", base, dateDir)
	}
	name := nextExportFilename(base, hasBase, maxSuffix)
	return exportPathPlan{
		AbsolutePath: filepath.Join(dateDir, name),
		DateDir:      dateDir,
		Basename:     strings.TrimSuffix(name, ".md"),
	}, nil
}

func exportExistingMatches(dateDir, base string) (hasBase bool, maxSuffix int, err error) {
	maxSuffix = -1
	entries, err := os.ReadDir(dateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, -1, nil
		}
		return false, -1, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if stem == base {
			hasBase = true
			if maxSuffix < 0 {
				maxSuffix = 0
			}
			continue
		}
		m := exportSuffixFileRE.FindStringSubmatch(name)
		if len(m) != 3 || m[1] != base {
			continue
		}
		n, convErr := strconv.Atoi(m[2])
		if convErr != nil {
			continue
		}
		hasBase = true
		if n > maxSuffix {
			maxSuffix = n
		}
	}
	return hasBase, maxSuffix, nil
}

func nextExportFilename(base string, hasBase bool, maxSuffix int) string {
	if !hasBase && maxSuffix < 0 {
		return base + ".md"
	}
	return fmt.Sprintf("%s-%d.md", base, maxSuffix+1)
}
