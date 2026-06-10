package sdk

func WebSearchInfo(query string) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", 0, 0)
}

func WebSearchNInfo(query string, maxResults int) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", maxResults, 0)
}

func WebSearchWithTimeoutInfo(query string, secs int) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", 0, secs)
}

func WebSearchEngineInfo(query, engine string) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, 0, 0)
}

func WebSearchEngineNInfo(query, engine string, maxResults int) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, maxResults, 0)
}

func WebSearchEngineTimeoutInfo(query, engine string, secs int) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, 0, secs)
}

func WebSearchNTimeoutInfo(query string, maxResults, secs int) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", maxResults, secs)
}

func WebSearchEngineNTimeoutInfo(query, engine string, maxResults, secs int) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, maxResults, secs)
}

func webSearchInfoCall(query, engine string, maxResults, secs int) (WebSearchResult, error) {
	raw, err := webSearchCall(query, engine, maxResults, secs)
	if err != nil {
		return WebSearchResult{}, err
	}
	return parseWebSearchResult([]byte(raw))
}
