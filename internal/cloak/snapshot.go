package cloak

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const maxSnapshotLinks = 1000

// ParsedSnapshot is the DOM-derived part of a browser snapshot. It is kept
// separate from Snapshot because the Node sidecar only transports HTML; Go
// owns the interpretation of that HTML.
type ParsedSnapshot struct {
	Title string
	Text  string
	Links []Link
}

// ParseSnapshotHTML extracts the useful snapshot content in Go. Keeping this
// function on the package surface also lets host-level tests exercise the
// exact parser used by Client without starting a browser process.
func ParseSnapshotHTML(rawHTML, rawBaseURL string) ParsedSnapshot {
	return ParsedSnapshot{
		Title: extractSnapshotTitle(rawHTML),
		Text:  extractSnapshotText(rawHTML),
		Links: extractSnapshotLinks(rawHTML, rawBaseURL),
	}
}

func extractSnapshotTitle(rawHTML string) string {
	if strings.TrimSpace(rawHTML) == "" {
		return ""
	}
	document, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	var title string
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node == nil || title != "" {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "title") {
			title = compactNodeText(node)
			return
		}
		for child := node.FirstChild; child != nil && title == ""; child = child.NextSibling {
			visit(child)
		}
	}
	visit(document)
	return truncateSnapshotText(title, 500)
}

func extractSnapshotText(rawHTML string) string {
	if strings.TrimSpace(rawHTML) == "" {
		return ""
	}
	document, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	return compactNodeText(document)
}

// extractSnapshotLinks keeps DOM interpretation in Go. The Node sidecar only
// has to ask Playwright for the current document; Solomon owns filtering,
// normalization and the shape consumed by its adapters.
func extractSnapshotLinks(rawHTML, rawBaseURL string) []Link {
	if strings.TrimSpace(rawHTML) == "" {
		return nil
	}
	document, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}
	baseURL, _ := url.Parse(strings.TrimSpace(rawBaseURL))
	links := make([]Link, 0)
	seen := make(map[string]struct{})
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node == nil || len(links) >= maxSnapshotLinks {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			href := attribute(node, "href")
			if resolved, ok := resolveSnapshotURL(href, baseURL); ok {
				if _, alreadySeen := seen[resolved]; !alreadySeen && resolved != strings.TrimSpace(rawBaseURL) {
					seen[resolved] = struct{}{}
					title := compactNodeText(node)
					if title == "" {
						title = compactNodeTextValue(attribute(node, "aria-label"))
					}
					if title == "" {
						title = compactNodeTextValue(attribute(node, "title"))
					}
					links = append(links, Link{
						Title:   truncateSnapshotText(title, 500),
						URL:     resolved,
						Snippet: truncateSnapshotText(snapshotContainerText(node), 1500),
					})
				}
			}
		}
		for child := node.FirstChild; child != nil && len(links) < maxSnapshotLinks; child = child.NextSibling {
			visit(child)
		}
	}
	visit(document)
	return links
}

func attribute(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func resolveSnapshotURL(raw string, base *url.URL) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if base != nil {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	return parsed.String(), true
}

func compactNodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var parts []string
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.ElementNode {
			switch strings.ToLower(current.Data) {
			case "script", "style", "noscript", "template", "svg":
				return
			}
		}
		if current.Type == html.TextNode {
			if value := strings.TrimSpace(current.Data); value != "" {
				parts = append(parts, value)
			}
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return compactNodeTextValue(strings.Join(parts, " "))
}

func snapshotContainerText(node *html.Node) string {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		switch strings.ToLower(parent.Data) {
		case "article", "li", "h2", "h3", "div":
			if text := compactNodeText(parent); text != "" {
				return text
			}
		}
	}
	return compactNodeText(node)
}

func compactNodeTextValue(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncateSnapshotText(value string, max int) string {
	value = compactNodeTextValue(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
