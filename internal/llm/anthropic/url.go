package anthropic

import (
	"net/url"
	"strings"
)

func MessagesURL(base string) string {
	return messagesURL(base, false)
}

func MessagesURLForAuth(base string, auth Auth) string {
	return messagesURL(base, auth.OAuth())
}

func messagesURL(base string, oauth bool) string {
	base = strings.TrimSpace(base)
	base = strings.TrimSuffix(base, "/")
	if strings.HasSuffix(base, "/v1/messages") {
		if oauth {
			return appendBetaQuery(base)
		}
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		base = base + "/messages"
	} else {
		base = base + "/v1/messages"
	}
	if oauth {
		return appendBetaQuery(base)
	}
	return base
}

func appendBetaQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if strings.Contains(raw, "?") {
			return raw + "&beta=true"
		}
		return raw + "?beta=true"
	}
	q := u.Query()
	q.Set("beta", "true")
	u.RawQuery = q.Encode()
	return u.String()
}
