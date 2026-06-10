package sdk

func WebSearch(query string) (string, error) {
	return webSearchCall(query, "", 0, 0)
}

func WebSearchN(query string, maxResults int) (string, error) {
	return webSearchCall(query, "", maxResults, 0)
}

func WebSearchWithTimeout(query string, secs int) (string, error) {
	return webSearchCall(query, "", 0, secs)
}

func WebSearchEngine(query, engine string) (string, error) {
	return webSearchCall(query, engine, 0, 0)
}

func WebSearchEngineN(query, engine string, maxResults int) (string, error) {
	return webSearchCall(query, engine, maxResults, 0)
}

func WebSearchEngineTimeout(query, engine string, secs int) (string, error) {
	return webSearchCall(query, engine, 0, secs)
}

func WebSearchNTimeout(query string, maxResults, secs int) (string, error) {
	return webSearchCall(query, "", maxResults, secs)
}

func WebSearchEngineNTimeout(query, engine string, maxResults, secs int) (string, error) {
	return webSearchCall(query, engine, maxResults, secs)
}

func webSearchCall(query, engine string, maxResults, secs int) (string, error) {
	args := map[string]any{"query": query}
	if engine != "" {
		args["engine"] = engine
	}
	if maxResults > 0 {
		args["maxResults"] = maxResults
	}
	if secs > 0 {
		args["timeoutSeconds"] = secs
	}
	raw, err := callTool("webSearch", args)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func FetchWeb(url string) (string, error) {
	r, err := FetchWebInfo(url)
	if err != nil {
		return "", err
	}
	return r.Markdown, nil
}

func FetchWebWithTimeout(url string, secs int) (string, error) {
	r, err := fetchWebCall(url, secs)
	if err != nil {
		return "", err
	}
	return r.Markdown, nil
}

func FetchWebInfo(url string) (FetchWebResult, error) {
	return fetchWebCall(url, 0)
}

func FetchWebInfoWithTimeout(url string, secs int) (FetchWebResult, error) {
	return fetchWebCall(url, secs)
}

func fetchWebCall(url string, secs int) (FetchWebResult, error) {
	args := map[string]any{"url": url}
	if secs > 0 {
		args["timeoutSeconds"] = secs
	}
	raw, err := callTool("fetchWeb", args)
	if err != nil {
		return FetchWebResult{}, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return FetchWebResult{}, err
	}
	return parseFetchWebResult(m), nil
}
