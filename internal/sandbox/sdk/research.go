package sdk

func DeepResearch(query, category string) (map[string]any, error) {
	args := map[string]any{"query": query}
	if category != "" {
		args["category"] = category
	}
	raw, err := callTool("deepResearch", args)
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func ResearchStatus(jobID string) (map[string]any, error) {
	raw, err := callTool("researchStatus", map[string]any{"jobId": jobID})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}
