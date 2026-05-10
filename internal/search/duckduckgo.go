package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type duckduckgoEngine struct{}

func (duckduckgoEngine) Search(ctx context.Context, req Request) (Response, error) {
	max := req.MaxResults
	if max <= 0 {
		max = 10
	}
	if max > 50 {
		max = 50
	}

	form := url.Values{}
	form.Set("q", req.Query)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return Response{}, fmt.Errorf("duckduckgo: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Solomon-webSearch/1.0)")
	httpReq.Header.Set("Accept", "text/html,*/*")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("duckduckgo: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("duckduckgo: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("duckduckgo: HTTP %d", resp.StatusCode)
	}

	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Response{}, fmt.Errorf("duckduckgo: parse html: %w", err)
	}
	raw := duckduckgoCollectHits(root)
	seen := map[string]struct{}{}
	out := Response{Hits: make([]Hit, 0)}
	for _, h := range raw {
		link := strings.TrimSpace(h.URL)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out.Hits = append(out.Hits, h)
		if len(out.Hits) >= max+1 {
			break
		}
	}
	if len(out.Hits) > max {
		out.HasMore = true
		out.Hits = out.Hits[:max]
	}
	if len(out.Hits) == 0 {
		return Response{}, fmt.Errorf("duckduckgo: no parsed results (markup may have changed)")
	}
	return out, nil
}

func duckduckgoCollectHits(root *html.Node) []Hit {
	var hits []Hit
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "a") {
			cls := duckHTMLAttr(n, "class")
			href := duckHTMLAttr(n, "href")
			if duckHTMLHasToken(cls, "result__a") && href != "" {
				link := duckduckgoNormalizeHref(href)
				title := strings.TrimSpace(htmlNodeInnerText(n))
				if link != "" && title != "" {
					snip := duckduckgoSnippetNearby(n)
					hits = append(hits, Hit{Title: title, URL: link, Snippet: snip})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return hits
}

func duckduckgoSnippetNearby(link *html.Node) string {
	if link == nil {
		return ""
	}
	for depth := 0; depth < 4 && link != nil; depth++ {
		for s := link.NextSibling; s != nil; s = s.NextSibling {
			if t := duckduckgoFindSnippetInTree(s); t != "" {
				return t
			}
		}
		link = link.Parent
		if link != nil && link.NextSibling != nil {
			for s := link.NextSibling; s != nil; s = s.NextSibling {
				if t := duckduckgoFindSnippetInTree(s); t != "" {
					return t
				}
			}
		}
		if link != nil {
			link = link.Parent
		}
	}
	return ""
}

func duckduckgoFindSnippetInTree(root *html.Node) string {
	if root == nil {
		return ""
	}
	if root.Type == html.ElementNode && duckHTMLHasToken(duckHTMLAttr(root, "class"), "result__snippet") {
		return strings.TrimSpace(htmlNodeInnerText(root))
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if t := duckduckgoFindSnippetInTree(c); t != "" {
			return t
		}
	}
	return ""
}

func duckHTMLAttr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return a.Val
		}
	}
	return ""
}

func duckHTMLHasToken(raw, tok string) bool {
	for _, p := range strings.Fields(raw) {
		if strings.EqualFold(p, tok) {
			return true
		}
	}
	return false
}

func htmlNodeInnerText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func duckduckgoNormalizeHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
		href = "https://duckduckgo.com" + href
	}
	u, err := url.Parse(href)
	if err != nil || u.Host == "" {
		return ""
	}
	if ud := u.Query().Get("uddg"); ud != "" {
		dec, err := url.QueryUnescape(ud)
		if err == nil {
			dec = strings.TrimSpace(dec)
			if dec != "" && strings.HasPrefix(strings.ToLower(dec), "http") {
				return dec
			}
		}
	}
	if !strings.HasPrefix(strings.ToLower(u.Scheme), "http") {
		return ""
	}
	return u.String()
}

func init() {
	Register("duckduckgo", duckduckgoEngine{})
}
