package sdk

func readFileCall(path string, start, end *int) (ReadResult, error) {
	args := map[string]any{"path": path}
	if start != nil {
		args["startLine"] = *start
	}
	if end != nil {
		args["endLine"] = *end
	}
	raw, err := callTool("readFile", args)
	if err != nil {
		return ReadResult{}, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return ReadResult{}, err
	}
	return parseReadResult(m), nil
}

func readFileFromLine(path string, start int) (ReadResult, error) {
	if start < 1 {
		start = 1
	}
	return readFileCall(path, &start, nil)
}

func readFileUntilLine(path string, end int) (ReadResult, error) {
	if end < 1 {
		end = 1
	}
	return readFileCall(path, nil, &end)
}
