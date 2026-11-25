package llm

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

var legacyResetTime = regexp.MustCompile(`(?im)(?:^reset:\s*|\bresets?\s+)(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}(?::\d{2})?)`)
var httpRateLimit = regexp.MustCompile(`(?i)\b(?:http[: ]+429|429 too many requests)\b`)
var relativeResetTime = regexp.MustCompile(`(?im)^reset:[ \t]+in (\d+) seconds?`)

func RateLimitDetails(err error, observed time.Time) (string, time.Time, bool) {
	if err == nil {
		return "", time.Time{}, false
	}
	raw := err.Error()
	var httpErr *transport.ProviderHTTPError
	httpLimited := errors.As(err, &httpErr) && httpErr.StatusCode == 429
	if httpErr != nil {
		raw = httpErr.Message
	}
	notice := UserFacingAPIError(err)
	lower := strings.ToLower(notice)
	if !httpLimited && !strings.Contains(lower, "usage_limit_reached") && !strings.Contains(lower, "usage limit") &&
		!strings.Contains(lower, "rate_limit_error") && !strings.Contains(lower, "rate limit") &&
		!strings.Contains(lower, "rate-limit") && !httpRateLimit.MatchString(raw+"\n"+notice) {
		return "", time.Time{}, false
	}
	if body, _, ok := extractAPIErrorPayload(raw); ok {
		if body.ResetsAt > 0 {
			return notice, time.Unix(body.ResetsAt, 0), true
		}
		if body.ResetsInSeconds > 0 {
			return notice, observed.Add(time.Duration(body.ResetsInSeconds) * time.Second), true
		}
	}
	reset := RateLimitResetTime(notice)
	if match := relativeResetTime.FindStringSubmatch(notice); reset.IsZero() && len(match) != 0 {
		if seconds, err := strconv.ParseInt(match[1], 10, 32); err == nil {
			reset = observed.Add(time.Duration(seconds) * time.Second)
		}
	}
	if reset.IsZero() && httpLimited && httpErr.RetryAfter > 0 {
		reset = observed.Add(httpErr.RetryAfter)
	}
	return notice, reset, true
}

func RateLimitResetTime(notice string) time.Time {
	if body, _, ok := extractAPIErrorPayload(notice); ok && body.ResetsAt > 0 {
		return time.Unix(body.ResetsAt, 0)
	}
	match := legacyResetTime.FindStringSubmatch(notice)
	if len(match) == 0 {
		return time.Time{}
	}
	stamp := strings.Replace(match[1], "T", " ", 1)
	if len(stamp) == 16 {
		stamp += ":00"
	}
	reset, _ := time.ParseInLocation("2006-01-02 15:04:05", stamp, time.Local)
	return reset
}
