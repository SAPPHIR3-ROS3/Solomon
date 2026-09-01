package sdk

func DocsRetrieval(query, intent string) (string, error) {
	return docsCall(query, intent)
}

func DocsSearch(query, intent string) (string, error) {
	return docsCall(query, intent)
}

func DocsArticle(path, intent string) (string, error) {
	return docsCall(path, intent)
}

func docsCall(query, intent string) (string, error) {
	raw, err := callTool("docsRetrieval", map[string]any{"query": query, "intent": intent})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
