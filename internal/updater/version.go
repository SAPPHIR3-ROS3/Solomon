package updater

import (
	"strconv"
	"strings"
)

func parseReleaseKey(tag string) []int {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	if tag == "" {
		return nil
	}
	var key []int
	for _, p := range strings.Split(tag, ".") {
		n, rest := parseLeadingDigits(p)
		if n < 0 {
			continue
		}
		key = append(key, n)
		if rest != "" {
			key = append(key, int(rest[0]))
		}
	}
	return key
}

func parseLeadingDigits(s string) (int, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return -1, s
	}
	n, _ := strconv.Atoi(s[:i])
	return n, s[i:]
}

func compareReleaseKeys(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var xa, xb int
		if i < len(a) {
			xa = a[i]
		}
		if i < len(b) {
			xb = b[i]
		}
		if xa != xb {
			return xa - xb
		}
	}
	return 0
}

func IsNewerRelease(latestTag, current string) bool {
	latest := parseReleaseKey(latestTag)
	if len(latest) == 0 {
		return false
	}
	current = strings.TrimSpace(current)
	if current == "" || current == "dev" || strings.Contains(current, "-dev-") {
		return true
	}
	cur := parseReleaseKey(current)
	if len(cur) == 0 {
		return true
	}
	return compareReleaseKeys(latest, cur) > 0
}
