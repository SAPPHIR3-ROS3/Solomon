package tools

func BuildDeferredToolDump() (string, error) {
	b := &dumpBuilder{}
	for _, tool := range deferredCatalog() {
		switch tool.Name {
		case "createDir":
			b.addBlock(tool.Name, tool.Description, "createDir(path, intent string)")
		case "httpRequest":
			b.addBlock(tool.Name, tool.Description, "httpRequest(url, method string, headers map[string]string, body, bodyBase64 string, timeoutSeconds int, maxBytes int64, intent string)")
		case "stat":
			b.addBlock(tool.Name, tool.Description, "stat(path, intent string)")
		case "copyFile":
			b.addBlock(tool.Name, tool.Description, "copyFile(path, destination, intent string)")
		}
	}
	if err := appendDeepResearchDump(b); err != nil {
		return "", err
	}
	if err := appendResearchStatusDump(b); err != nil {
		return "", err
	}
	if err := appendCreatePlanDump(b); err != nil {
		return "", err
	}
	if err := appendEditPlanDump(b); err != nil {
		return "", err
	}
	if err := appendBuildPlanDump(b); err != nil {
		return "", err
	}
	if err := appendAddTodoDump(b); err != nil {
		return "", err
	}
	if err := appendTodoListDump(b); err != nil {
		return "", err
	}
	if err := appendCheckTodoDump(b); err != nil {
		return "", err
	}
	if err := appendRemoveTodoDump(b); err != nil {
		return "", err
	}
	if err := appendCheckPlanDump(b); err != nil {
		return "", err
	}
	if err := appendDeletePlanDump(b); err != nil {
		return "", err
	}
	if err := appendShellDump(b); err != nil {
		return "", err
	}
	if err := appendReadFileDump(b); err != nil {
		return "", err
	}
	if err := appendFindDump(b); err != nil {
		return "", err
	}
	if err := appendListDirDump(b); err != nil {
		return "", err
	}
	if err := appendTreeDump(b); err != nil {
		return "", err
	}
	if err := appendEditFileDump(b); err != nil {
		return "", err
	}
	if err := appendFetchWebDump(b); err != nil {
		return "", err
	}
	if err := appendWebSearchDump(b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func BuildPlanningNativeToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendBuildPlanDump(b); err != nil {
		return "", err
	}
	return b.String(), nil
}
