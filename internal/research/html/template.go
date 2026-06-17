package html

import (
	"regexp"
	"strings"
)

var scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
var onEventRe = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
var jsURLRe = regexp.MustCompile(`(?i)javascript:`)

func sanitizeHTML(s string) string {
	s = scriptRe.ReplaceAllString(s, "")
	s = onEventRe.ReplaceAllString(s, "")
	s = jsURLRe.ReplaceAllString(s, "")
	return s
}

func css() string {
	return `
:root { --bg: #fafafa; --fg: #1a1a1a; --muted: #555; --accent: #2563eb; --card: #fff; --border: #e5e7eb; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #0f1117; --fg: #e8eaed; --muted: #9aa0a6; --accent: #8ab4f8; --card: #1a1d27; --border: #2d3139; }
}
* { box-sizing: border-box; }
body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: var(--bg); color: var(--fg); line-height: 1.65; }
.hero { padding: 2.5rem 1.5rem 1.5rem; max-width: 52rem; margin: 0 auto; border-bottom: 1px solid var(--border); }
.hero h1 { margin: 0 0 .5rem; font-size: 1.85rem; line-height: 1.25; }
.question { color: var(--muted); font-style: italic; margin: .25rem 0; }
.date { color: var(--muted); font-size: .9rem; margin: 0; }
main { max-width: 52rem; margin: 0 auto; padding: 1.5rem; }
.toc { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1.5rem; }
.toc h2 { margin: 0 0 .75rem; font-size: 1rem; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); }
.toc ul { list-style: none; padding: 0; margin: 0; }
.toc li { margin: .25rem 0; }
.toc-l3 { padding-left: 1rem; }
.toc-l4 { padding-left: 2rem; }
.toc a { color: var(--accent); text-decoration: none; }
.report h2, .report h3, .report h4 { margin-top: 1.75rem; }
.report p { margin: 1rem 0; }
.report a { color: var(--accent); }
.report table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
.report th, .report td { border: 1px solid var(--border); padding: .5rem .75rem; text-align: left; }
.report blockquote { border-left: 3px solid var(--accent); margin: 1rem 0; padding: .25rem 1rem; color: var(--muted); }
.report pre { background: var(--card); border: 1px solid var(--border); padding: 1rem; overflow-x: auto; border-radius: 6px; }
.report h2:last-of-type, .report h2:has(+ p) { }
.report h2:contains("TL;DR") { }
.sources { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid var(--border); }
.sources summary { cursor: pointer; font-weight: 600; }
.sources ol { padding-left: 1.25rem; }
.stats { margin-top: 2rem; color: var(--muted); font-size: .85rem; font-family: system-ui, sans-serif; }
article.report h2:last-of-type { margin-top: 2.5rem; padding: 1rem 1.25rem; background: var(--card); border-left: 4px solid var(--accent); border-radius: 0 8px 8px 0; }
`
}

func HasTLDRSection(md string) bool {
	return strings.Contains(strings.ToLower(md), "## tl;dr")
}
