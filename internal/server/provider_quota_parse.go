package server

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func quotaBarsFromPayload(payload any, kind string) []apiQuotaBar {
	switch kind {
	case "chatgpt":
		return quotaBarsFromChatGPTPayload(payload)
	case "claude":
		return quotaBarsFromClaudePayload(payload)
	case "cursor":
		return quotaBarsFromCursorPayload(payload)
	case "openrouter":
		return quotaBarsFromOpenRouterPayload(payload)
	default:
		return quotaBarsFromGenericPayload(payload)
	}
}

func quotaBarsFromChatGPTPayload(payload any) []apiQuotaBar {
	rateLimit := mapChild(payload, "rate_limit")
	if rateLimit == nil {
		return nil
	}
	var bars []apiQuotaBar
	for _, window := range []struct {
		key      string
		fallback string
	}{
		{key: "primary_window", fallback: "Primary limit"},
		{key: "secondary_window", fallback: "Secondary limit"},
	} {
		if value := mapChild(rateLimit, window.key); value != nil {
			if bar, ok := quotaBarFromWindow(value, window.fallback); ok {
				bars = append(bars, bar)
			}
		}
	}
	if spend := mapPath(payload, "spend_control", "individual_limit"); spend != nil {
		if bar, ok := quotaBarFromUsage(spend, "Spend control"); ok {
			bars = append(bars, bar)
		} else if bar, ok := quotaBarFromWindow(spend, "Spend control"); ok {
			bar.Label = "Spend control"
			bars = append(bars, bar)
		}
	}
	return uniqueQuotaBars(bars)
}

func quotaBarsFromClaudePayload(payload any) []apiQuotaBar {
	var bars []apiQuotaBar
	for _, window := range []struct {
		key   string
		label string
	}{
		{key: "five_hour", label: "5-hour limit"},
		{key: "seven_day", label: "Weekly limit"},
		{key: "seven_day_sonnet", label: "Weekly Sonnet limit"},
		{key: "seven_day_opus", label: "Weekly Opus limit"},
		{key: "seven_day_oauth_apps", label: "Weekly OAuth apps limit"},
	} {
		value := mapChild(payload, window.key)
		if value == nil {
			continue
		}
		if bar, ok := quotaBarFromWindow(value, window.label); ok {
			bar.Label = window.label
			bar.Percent = normalizeClaudePercent(bar.Percent)
			bars = append(bars, bar)
		}
	}
	if extra := mapChild(payload, "extra_usage"); extra != nil {
		if bar, ok := quotaBarFromUsage(extra, "Extra usage"); ok {
			bars = append(bars, bar)
		}
	}
	return uniqueQuotaBars(bars)
}

func quotaBarsFromCursorPayload(payload any) []apiQuotaBar {
	var bars []apiQuotaBar
	if plan := mapChild(payload, "planUsage"); plan != nil {
		if bar, ok := quotaBarFromUsage(plan, "Plan usage"); ok {
			bars = append(bars, bar)
		}
		for _, metric := range []struct {
			key   string
			label string
		}{
			{key: "autoPercentUsed", label: "Auto models"},
			{key: "apiPercentUsed", label: "API models"},
			{key: "totalPercentUsed", label: "Total usage"},
		} {
			if percent, ok := numberField(asMap(plan), metric.key); ok {
				bars = append(bars, apiQuotaBar{Label: metric.label, Percent: clampPercent(percent)})
			}
		}
	}
	if plan := mapPath(payload, "individualUsage", "plan"); plan != nil {
		if bar, ok := quotaBarFromUsage(plan, "Plan usage"); ok {
			bars = append(bars, bar)
		}
	}
	if onDemand := mapPath(payload, "individualUsage", "onDemand"); onDemand != nil {
		if bar, ok := quotaBarFromUsage(onDemand, "On-demand usage"); ok {
			bars = append(bars, bar)
		}
	}
	if spend := mapChild(payload, "spendLimitUsage"); spend != nil {
		if bar, ok := quotaBarFromUsage(spend, "On-demand budget"); ok {
			bars = append(bars, bar)
		}
	}
	if len(bars) == 0 {
		if bar, ok := quotaBarFromUsage(asMap(payload), "Plan usage"); ok {
			bars = append(bars, bar)
		}
	}
	return uniqueQuotaBars(bars)
}

func quotaBarsFromOpenRouterPayload(payload any) []apiQuotaBar {
	data := mapChild(payload, "data")
	if data == nil {
		return nil
	}
	used, hasUsed := numberField(asMap(data), "total_usage")
	credits, hasCredits := numberField(asMap(data), "total_credits")
	if !hasUsed || !hasCredits || credits <= 0 {
		return nil
	}
	return []apiQuotaBar{{
		Label:   "Credits usage",
		Percent: clampPercent(used / credits * 100),
		Detail:  fmt.Sprintf("$%.2f / $%.2f", used, credits),
	}}
}

func quotaBarsFromGenericPayload(payload any) []apiQuotaBar {
	if rateLimit := mapChild(payload, "rate_limit"); rateLimit != nil {
		var bars []apiQuotaBar
		for _, key := range []string{"primary_window", "secondary_window"} {
			if value := mapChild(rateLimit, key); value != nil {
				if bar, ok := quotaBarFromWindow(value, "Plan usage"); ok {
					bars = append(bars, bar)
				}
			}
		}
		if len(bars) > 0 {
			return uniqueQuotaBars(bars)
		}
	}
	if bar, ok := quotaBarFromUsage(asMap(payload), "Plan usage"); ok {
		return []apiQuotaBar{bar}
	}
	return nil
}

func quotaBarFromWindow(window any, fallback string) (apiQuotaBar, bool) {
	m, ok := window.(map[string]any)
	if !ok {
		return apiQuotaBar{}, false
	}
	percent := numberFieldValue(m, "used_percent", "usedPercentage", "utilization", "percent", "percentage", "pct", "usage_percent")
	if percent < 0 {
		used, hasUsed := numberField(m, "used", "used_usd", "requests_used", "tokens_used", "tokensUsed", "consumed")
		limit, hasLimit := numberField(m, "limit", "limit_usd", "requests_limit", "tokens_limit", "tokensLimit", "allotment", "cap")
		if hasUsed && hasLimit && limit > 0 {
			percent = used / limit * 100
		}
	}
	if percent < 0 {
		return apiQuotaBar{}, false
	}
	return apiQuotaBar{
		Label:   windowLabel(m, fallback),
		Percent: clampPercent(percent),
		Detail:  resetDetail(m),
	}, true
}

func quotaBarFromUsage(value any, label string) (apiQuotaBar, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return apiQuotaBar{}, false
	}
	percent := numberFieldValue(m, "totalPercentUsed", "total_percent_used", "percentUsed", "usedPercent", "percent_used", "used_percentage", "utilization", "percent", "percentage", "pct")
	if percent >= 0 {
		return apiQuotaBar{Label: label, Percent: clampPercent(percent), Detail: resetDetail(m)}, true
	}
	used, hasUsed := numberField(m, "used", "totalSpend", "includedSpend", "consumed", "numRequests", "requests_used", "tokens_used")
	limit, hasLimit := numberField(m, "limit", "individualLimit", "pooledLimit", "maxRequestUsage", "requests_limit", "tokens_limit", "allotment", "cap")
	if hasUsed && hasLimit && limit > 0 {
		return apiQuotaBar{Label: label, Percent: clampPercent(used / limit * 100), Detail: resetDetail(m)}, true
	}
	remaining, hasRemaining := numberField(m, "remaining", "individualRemaining", "pooledRemaining", "left")
	if hasRemaining && hasLimit && limit > 0 {
		return apiQuotaBar{Label: label, Percent: clampPercent((limit - remaining) / limit * 100), Detail: resetDetail(m)}, true
	}
	return apiQuotaBar{}, false
}

func mapChild(value any, key string) any {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	for name, child := range m {
		if strings.EqualFold(name, key) {
			return child
		}
	}
	return nil
}

func mapPath(value any, keys ...string) any {
	for _, key := range keys {
		value = mapChild(value, key)
		if value == nil {
			return nil
		}
	}
	return value
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func numberFieldValue(m map[string]any, names ...string) float64 {
	for _, name := range names {
		if value, ok := numberField(m, name); ok {
			return value
		}
	}
	return -1
}

func numberField(m map[string]any, names ...string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	for key, value := range m {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				return numericValue(value)
			}
		}
	}
	return 0, false
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(number, "$")), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func windowLabel(m map[string]any, fallback string) string {
	seconds, ok := numberField(m, "limit_window_seconds", "window_seconds", "duration_seconds")
	if ok {
		switch int64(seconds) {
		case 18000:
			return "5-hour limit"
		case 604800:
			return "Weekly limit"
		}
	}
	if label := quotaStringField(m, "label", "title", "name"); label != "" {
		return label
	}
	return fallback
}

func resetDetail(m map[string]any) string {
	seconds, hasSeconds := numberField(m, "reset_after_seconds", "resetAfterSeconds", "resets_in_seconds", "resetsInSeconds")
	var resetAt time.Time
	if raw := quotaStringField(m, "resets_at", "reset_at", "resetsAt", "resetAt"); raw != "" {
		resetAt, _ = time.Parse(time.RFC3339Nano, raw)
		if resetAt.IsZero() {
			if timestamp, err := strconv.ParseFloat(raw, 64); err == nil {
				resetAt = quotaTimestamp(timestamp)
			}
		}
	}
	if resetAt.IsZero() {
		if timestamp, ok := numberField(m, "reset_at", "resetAt", "resets_at", "resetsAt"); ok {
			resetAt = quotaTimestamp(timestamp)
		}
	}
	if !hasSeconds && !resetAt.IsZero() {
		seconds = time.Until(resetAt).Seconds()
		hasSeconds = seconds > 0
	}
	if !resetAt.IsZero() {
		detail := "reset " + resetAt.Local().Format("2006-01-02 15:04")
		if hasSeconds && seconds > 0 {
			return detail + " (" + formatDuration(int64(seconds)) + ")"
		}
		return detail
	}
	if hasSeconds && seconds > 0 {
		return "reset " + formatDuration(int64(seconds))
	}
	return ""
}

func quotaTimestamp(timestamp float64) time.Time {
	if timestamp > 1e12 {
		timestamp /= 1000
	}
	if timestamp <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(timestamp), 0)
}

func quotaStringField(m map[string]any, names ...string) string {
	for key, value := range m {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				result, ok := value.(string)
				if ok {
					return strings.TrimSpace(result)
				}
			}
		}
	}
	return ""
}

func normalizeClaudePercent(value float64) float64 {
	if value >= 0 && value <= 1 {
		return clampPercent(value * 100)
	}
	return clampPercent(value)
}

func uniqueQuotaBars(bars []apiQuotaBar) []apiQuotaBar {
	seen := make(map[string]bool, len(bars))
	out := make([]apiQuotaBar, 0, len(bars))
	for _, bar := range bars {
		key := bar.Label + ":" + fmt.Sprintf("%.4f", bar.Percent)
		if bar.Label == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, bar)
	}
	return out
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("in %ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("in %dm", minutes)
	}
	hours := minutes / 60
	minutes %= 60
	if minutes == 0 {
		return fmt.Sprintf("in %dh", hours)
	}
	return fmt.Sprintf("in %dh %dm", hours, minutes)
}

func clampPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// QuotaBarForTest is a narrow test view of a provider quota bar.
type QuotaBarForTest struct {
	Label   string
	Percent float64
}

// QuotaBarsForTest exposes the pure parser to external package tests.
func QuotaBarsForTest(payload any, kind string) []QuotaBarForTest {
	bars := quotaBarsFromPayload(payload, kind)
	out := make([]QuotaBarForTest, len(bars))
	for i, bar := range bars {
		out[i] = QuotaBarForTest{Label: bar.Label, Percent: bar.Percent}
	}
	return out
}
