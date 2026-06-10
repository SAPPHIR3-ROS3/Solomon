package sdk

func DocsRetrieval(query string) (string, error) {
	return docsCall(query)
}

func DocsSearch(query string) (string, error) {
	return docsCall(query)
}

func DocsArticle(path string) (string, error) {
	return docsCall(path)
}

func docsCall(query string) (string, error) {
	raw, err := callTool("docsRetrieval", map[string]any{"query": query})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
