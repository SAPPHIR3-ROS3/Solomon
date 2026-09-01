package sdk

func WebSearch(query, intent string) (string, error) {
	return webSearchCall(query, "", 0, 0, intent)
}

func WebSearchN(query string, maxResults int, intent string) (string, error) {
	return webSearchCall(query, "", maxResults, 0, intent)
}

func WebSearchWithTimeout(query string, secs int, intent string) (string, error) {
	return webSearchCall(query, "", 0, secs, intent)
}

func WebSearchEngine(query, engine, intent string) (string, error) {
	return webSearchCall(query, engine, 0, 0, intent)
}

func WebSearchEngineN(query, engine string, maxResults int, intent string) (string, error) {
	return webSearchCall(query, engine, maxResults, 0, intent)
}

func WebSearchEngineTimeout(query, engine string, secs int, intent string) (string, error) {
	return webSearchCall(query, engine, 0, secs, intent)
}

func WebSearchNTimeout(query string, maxResults, secs int, intent string) (string, error) {
	return webSearchCall(query, "", maxResults, secs, intent)
}

func WebSearchEngineNTimeout(query, engine string, maxResults, secs int, intent string) (string, error) {
	return webSearchCall(query, engine, maxResults, secs, intent)
}

func webSearchCall(query, engine string, maxResults, secs int, intent string) (string, error) {
	args := map[string]any{"query": query, "intent": intent}
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

func FetchWeb(url, intent string) (string, error) {
	r, err := FetchWebInfo(url, intent)
	if err != nil {
		return "", err
	}
	return r.Markdown, nil
}

func FetchWebWithTimeout(url string, secs int, intent string) (string, error) {
	r, err := fetchWebCall(url, secs, intent)
	if err != nil {
		return "", err
	}
	return r.Markdown, nil
}

func FetchWebInfo(url, intent string) (FetchWebResult, error) {
	return fetchWebCall(url, 0, intent)
}

func FetchWebInfoWithTimeout(url string, secs int, intent string) (FetchWebResult, error) {
	return fetchWebCall(url, secs, intent)
}

func fetchWebCall(url string, secs int, intent string) (FetchWebResult, error) {
	args := map[string]any{"url": url, "intent": intent}
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
