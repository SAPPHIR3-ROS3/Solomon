package research

import "strings"

var lowQualityMarkers = []string{
	"insufficient to",
	"content is insufficient",
	"no substantive data",
	"does not contain",
	"not relevant to",
	"no relevant information",
	"unable to extract",
	"completely unrelated",
	"boilerplate",
	"footer text",
	"cookie consent",
	"cookie banner",
	"cookie notice",
	"copyright notice",
	"copyright footer",
	"all rights reserved",
}

func isLowQuality(summary string) bool {
	if strings.TrimSpace(summary) == "" {
		return true
	}
	low := strings.ToLower(summary)
	for _, m := range lowQualityMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

func formatFindings(findings []Finding) string {
	var parts []string
	for i, f := range findings {
		title := f.Title
		if title == "" {
			title = f.URL
		}
		content := f.Summary
		if content == "" && f.Evidence != "" {
			content = f.Evidence
			if len(content) > 1000 {
				content = content[:1000]
			}
		}
		if content == "" {
			content = "(no content)"
		}
		parts = append(parts, "**Finding "+itoa(i+1)+"** — ["+title+"]("+f.URL+")\n"+content)
	}
	return strings.Join(parts, "\n\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func truncateContent(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	truncated := content[:maxChars]
	if lastPara := strings.LastIndex(truncated, "\n\n"); lastPara > maxChars*8/10 {
		return truncated[:lastPara]
	}
	return truncated
}
