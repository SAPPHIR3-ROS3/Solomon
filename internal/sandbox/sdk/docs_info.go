package sdk

func DocsRetrievalInfo(query, intent string) (DocsResult, error) {
	return docsInfoCall(query, intent)
}

func DocsSearchInfo(query, intent string) (DocsResult, error) {
	return docsInfoCall(query, intent)
}

func DocsArticleInfo(path, intent string) (DocsResult, error) {
	return docsInfoCall(path, intent)
}

func docsInfoCall(query, intent string) (DocsResult, error) {
	raw, err := callTool("docsRetrieval", map[string]any{"query": query, "intent": intent})
	if err != nil {
		return DocsResult{}, err
	}
	return parseDocsResult(raw)
}
