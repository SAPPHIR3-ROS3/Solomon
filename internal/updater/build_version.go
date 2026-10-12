package updater

import (
	"regexp"
	"strings"
)

var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

func releaseBase(value string) string {
	value = strings.TrimSpace(value)
	if marker := strings.Index(value, "-dev"); marker >= 0 {
		value = value[:marker]
	}
	if !releaseVersion.MatchString(value) {
		return ""
	}
	return "v" + strings.TrimPrefix(value, "v")
}

// DevelopmentBuildVersion selects the newest known release baseline while
// retaining development identity for untagged, modified, or older checkouts.
func DevelopmentBuildVersion(exact string, dirty bool, candidates ...string) string {
	base := releaseBase(exact)
	for _, candidate := range candidates {
		candidate = releaseBase(candidate)
		if candidate != "" && (base == "" || IsNewerRelease(candidate, base)) {
			base = candidate
		}
	}
	if base == "" {
		return "dev"
	}
	if !dirty && base == releaseBase(exact) {
		return base
	}
	return base + "-dev"
}
