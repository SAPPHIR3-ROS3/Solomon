package llm

import (
	"regexp"
	"strconv"
)

var messageHTTPStatus = regexp.MustCompile(`(?i)\b(?:http(?:/\d(?:\.\d)?)?(?:\s+status(?:\s+code)?)?|status(?:[ _]+code)?)\s*[:=_-]?\s*([1-5]\d{2})\b`)
var leadingHTTPStatus = regexp.MustCompile(`(?i)^\s*([45]\d{2})\s+(?:bad request|unauthorized|forbidden|not found|request timeout|too many requests|internal server error|bad gateway|service unavailable|gateway timeout)\b`)

func statusCodeFromMessage(message string) int {
	for _, pattern := range []*regexp.Regexp{messageHTTPStatus, leadingHTTPStatus} {
		if match := pattern.FindStringSubmatch(message); len(match) == 2 {
			status, _ := strconv.Atoi(match[1])
			return status
		}
	}
	return 0
}
