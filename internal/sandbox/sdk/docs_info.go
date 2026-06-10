package sdk

func DocsRetrievalInfo(query string) (DocsResult, error) {
	return docsInfoCall(query)
}

func DocsSearchInfo(query string) (DocsResult, error) {
	return docsInfoCall(query)
}

func DocsArticleInfo(path string) (DocsResult, error) {
	return docsInfoCall(path)
}

func docsInfoCall(query string) (DocsResult, error) {
	raw, err := callTool("docsRetrieval", map[string]any{"query": query})
	if err != nil {
		return DocsResult{}, err
	}
	return parseDocsResult(raw)
}
