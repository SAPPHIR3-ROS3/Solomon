package sdk

type globQuery struct {
	dir         string
	pattern     string
	headLimit   int
	timeoutSecs int
}

type grepTextQuery struct {
	dir, pattern, pathGlob, outputMode string
	caseInsensitive, multiline          bool
	context, contextBefore, contextAfter int
	headLimit, timeoutSecs              int
}

type grepPathsQuery struct {
	dir, pattern string
	caseInsensitive bool
	timeoutSecs     int
}

func globCall(q globQuery) ([]string, error) {
	args := map[string]any{"pattern": q.pattern, "files": true}
	if q.dir != "" {
		args["path"] = q.dir
	}
	if q.headLimit > 0 {
		args["headLimit"] = q.headLimit
	}
	if q.timeoutSecs > 0 {
		args["timeoutSeconds"] = q.timeoutSecs
	}
	raw, err := callTool("find", args)
	if err != nil {
		return nil, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return nil, err
	}
	return stringSliceField(m, "matches"), nil
}

func grepTextCall(q grepTextQuery) (string, error) {
	mode := q.outputMode
	if mode == "" {
		mode = "content"
	}
	args := map[string]any{
		"pattern": q.pattern, "files": false, "outputMode": mode,
	}
	if q.dir != "" {
		args["path"] = q.dir
	}
	if q.pathGlob != "" {
		args["pathGlob"] = q.pathGlob
	}
	if q.caseInsensitive {
		args["caseInsensitive"] = true
	}
	if q.multiline {
		args["multiline"] = true
	}
	if q.context > 0 {
		args["context"] = q.context
	}
	if q.contextBefore > 0 {
		args["contextBefore"] = q.contextBefore
	}
	if q.contextAfter > 0 {
		args["contextAfter"] = q.contextAfter
	}
	if q.headLimit > 0 {
		args["headLimit"] = q.headLimit
	}
	if q.timeoutSecs > 0 {
		args["timeoutSeconds"] = q.timeoutSecs
	}
	raw, err := callTool("find", args)
	if err != nil {
		return "", err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return "", err
	}
	return strField(m, "output"), nil
}

func grepPathsCall(q grepPathsQuery) ([]string, error) {
	args := map[string]any{
		"pattern": q.pattern, "files": false, "outputMode": "files_with_matches",
	}
	if q.dir != "" {
		args["path"] = q.dir
	}
	if q.caseInsensitive {
		args["caseInsensitive"] = true
	}
	if q.timeoutSecs > 0 {
		args["timeoutSeconds"] = q.timeoutSecs
	}
	raw, err := callTool("find", args)
	if err != nil {
		return nil, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return nil, err
	}
	return stringSliceField(m, "matches"), nil
}

func findRawCall(dir, pattern string, files bool, timeoutSecs int) (string, error) {
	args := map[string]any{"pattern": pattern, "files": files}
	if dir != "" {
		args["path"] = dir
	}
	if timeoutSecs > 0 {
		args["timeoutSeconds"] = timeoutSecs
	}
	raw, err := callTool("find", args)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func findInfoCall(dir, pattern string, files bool, timeoutSecs int) (FindResult, error) {
	args := map[string]any{"pattern": pattern, "files": files}
	if dir != "" {
		args["path"] = dir
	}
	if timeoutSecs > 0 {
		args["timeoutSeconds"] = timeoutSecs
	}
	raw, err := callTool("find", args)
	if err != nil {
		return FindResult{}, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return FindResult{}, err
	}
	return parseFindResult(m), nil
}

func grepLinesCall(q grepTextQuery) ([]GrepLine, error) {
	out, err := grepTextCall(q)
	if err != nil {
		return nil, err
	}
	return parseGrepLines(out), nil
}

func grepCountEntriesCall(q grepTextQuery) ([]GrepCountEntry, error) {
	q.outputMode = "count"
	out, err := grepTextCall(q)
	if err != nil {
		return nil, err
	}
	return parseGrepCountEntries(out), nil
}
