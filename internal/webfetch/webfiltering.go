package webfetch

func DomainBlockedForTest(host string, rules []string) (bool, string) {
	return domainBlocked(host, rules)
}

func ClassifyHTTPStatusForTest(status int, body []byte) string {
	return classifyHTTPStatus(status, body)
}

func RetryableFetchErrorForTest(err error) bool {
	return retryableFetchError(err)
}

func ResetSharedClientForTest() {
	clientMu.Lock()
	defer clientMu.Unlock()
	sharedClient = nil
	maxRedirects = 0
}
