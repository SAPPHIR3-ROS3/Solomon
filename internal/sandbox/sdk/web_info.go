package sdk

func WebSearchInfo(query, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", 0, 0, intent)
}

func WebSearchNInfo(query string, maxResults int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", maxResults, 0, intent)
}

func WebSearchWithTimeoutInfo(query string, secs int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", 0, secs, intent)
}

func WebSearchEngineInfo(query, engine, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, 0, 0, intent)
}

func WebSearchEngineNInfo(query, engine string, maxResults int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, maxResults, 0, intent)
}

func WebSearchEngineTimeoutInfo(query, engine string, secs int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, 0, secs, intent)
}

func WebSearchNTimeoutInfo(query string, maxResults, secs int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, "", maxResults, secs, intent)
}

func WebSearchEngineNTimeoutInfo(query, engine string, maxResults, secs int, intent string) (WebSearchResult, error) {
	return webSearchInfoCall(query, engine, maxResults, secs, intent)
}

func webSearchInfoCall(query, engine string, maxResults, secs int, intent string) (WebSearchResult, error) {
	raw, err := webSearchCall(query, engine, maxResults, secs, intent)
	if err != nil {
		return WebSearchResult{}, err
	}
	return parseWebSearchResult([]byte(raw))
}
