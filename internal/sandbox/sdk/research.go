package sdk

func DeepResearch(query, category, intent string) (map[string]any, error) {
	args := map[string]any{"query": query, "intent": intent}
	if category != "" {
		args["category"] = category
	}
	raw, err := callTool("deepResearch", args)
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func ResearchStatus(jobID, intent string) (map[string]any, error) {
	raw, err := callTool("researchStatus", map[string]any{"jobId": jobID, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}
