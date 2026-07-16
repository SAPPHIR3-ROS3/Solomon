//go:build automatic_role_scores

package roles

import (
	"regexp"
	"strings"
)

var (
	reDateSuffix   = regexp.MustCompile(`-\d{8}$`)
	reLatestSuffix = regexp.MustCompile(`-latest$`)
)

func NormalizeModelID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return ""
	}
	for {
		prev := id
		id = reDateSuffix.ReplaceAllString(id, "")
		id = reLatestSuffix.ReplaceAllString(id, "")
		if id == prev {
			break
		}
	}
	return id
}
