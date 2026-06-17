package sdk

import "encoding/json"

func ListDir(path string) ([]ListDirEntry, error) {
	r, err := ListDirInfo(path)
	if err != nil {
		return nil, err
	}
	return r.Entries, nil
}

func ListDirInfo(path string) (ListDirResult, error) {
	args := map[string]any{}
	if path != "" {
		args["path"] = path
	}
	raw, err := callTool("listDir", args)
	if err != nil {
		return ListDirResult{}, err
	}
	return parseListDirResult(raw)
}

func Tree(path string) (string, error) {
	r, err := TreeInfo(path)
	if err != nil {
		return "", err
	}
	return r.Tree, nil
}

func TreeDepth(path string, maxDepth int) (string, error) {
	args := map[string]any{}
	if path != "" {
		args["path"] = path
	}
	if maxDepth > 0 {
		args["maxDepth"] = maxDepth
	}
	raw, err := callTool("tree", args)
	if err != nil {
		return "", err
	}
	r, err := parseTreeResult(raw)
	if err != nil {
		return "", err
	}
	return r.Tree, nil
}

func TreeInfo(path string) (TreeResult, error) {
	args := map[string]any{}
	if path != "" {
		args["path"] = path
	}
	raw, err := callTool("tree", args)
	if err != nil {
		return TreeResult{}, err
	}
	return parseTreeResult(raw)
}

func parseListDirResult(raw json.RawMessage) (ListDirResult, error) {
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return ListDirResult{}, err
	}
	out := ListDirResult{
		Path:  strField(m, "path"),
		Count: intField(m, "count"),
	}
	if v, ok := m["entries"].([]any); ok {
		for _, item := range v {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out.Entries = append(out.Entries, ListDirEntry{
				Name: strField(row, "name"),
				Type: strField(row, "type"),
				Size: int64(intField(row, "size")),
			})
		}
	}
	return out, nil
}

func parseTreeResult(raw json.RawMessage) (TreeResult, error) {
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return TreeResult{}, err
	}
	return TreeResult{
		Path:      strField(m, "path"),
		Tree:      strField(m, "tree"),
		Entries:   intField(m, "entries"),
		Truncated: boolField(m, "truncated"),
	}, nil
}
