package research

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

var multiDashRe = regexp.MustCompile(`-+`)

func SlugFromQuery(query string) string {
	s := slugifyQuery(query)
	if s == "" {
		return "untitled-research"
	}
	const maxRunes = 48
	if r := []rune(s); len(r) > maxRunes {
		s = strings.Trim(string(r[:maxRunes]), "-")
	}
	return s
}

func slugifyQuery(query string) string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range query {
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
	return strings.Trim(multiDashRe.ReplaceAllString(b.String(), "-"), "-")
}

func TitleFromQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "untitled research"
	}
	const maxRunes = 80
	if r := []rune(query); len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return query
}

func ResolveUniqueSlug(projectHex, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "untitled-research"
	}
	dir, err := chatstore.ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	slug := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}
		htmlPath := filepath.Join(dir, slug+".html")
		if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
			return slug, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique research slug for %q", base)
}

func AppendTLDRSection(report, tldr string) string {
	tldr = strings.TrimSpace(tldr)
	if tldr == "" {
		return report
	}
	return strings.TrimSpace(report) + "\n\n## TL;DR\n\n" + tldr + "\n"
}
