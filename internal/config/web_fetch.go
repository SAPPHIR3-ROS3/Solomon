package config

import "strings"

const DefaultWebFetchMaxRedirects = 10

type WebFetchConfig struct {
	UserAgent      string   `toml:"user_agent,omitempty"`
	AcceptLanguage string   `toml:"accept_language,omitempty"`
	BlockedDomains []string `toml:"blocked_domains,omitempty"`
	MaxRedirects   int      `toml:"max_redirects,omitempty"`
}

func EffectiveWebFetchMaxRedirects(r *Root) int {
	if r == nil || r.WebFetch.MaxRedirects <= 0 {
		return DefaultWebFetchMaxRedirects
	}
	if r.WebFetch.MaxRedirects > 30 {
		return 30
	}
	return r.WebFetch.MaxRedirects
}

func EffectiveWebFetchUserAgent(r *Root) string {
	if r != nil {
		if s := strings.TrimSpace(r.WebFetch.UserAgent); s != "" {
			return s
		}
	}
	return ""
}

func EffectiveWebFetchAcceptLanguage(r *Root) string {
	if r != nil {
		if s := strings.TrimSpace(r.WebFetch.AcceptLanguage); s != "" {
			return s
		}
	}
	return "en-US,en;q=0.9"
}

func EffectiveWebFetchBlockedDomains(r *Root) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, d := range r.WebFetch.BlockedDomains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}
