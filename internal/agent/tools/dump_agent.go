package tools

func BuildAgentToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendDocsRetrievalDump(b); err != nil {
		return "", err
	}
	if err := appendSearchSkillDump(b); err != nil {
		return "", err
	}
	if err := appendLoadSkillDump(b); err != nil {
		return "", err
	}
	if err := appendSearchToolsDump(b); err != nil {
		return "", err
	}
	if err := appendOrchestrateDump(b); err != nil {
		return "", err
	}
	if err := appendSubagentDump(b); err != nil {
		return "", err
	}
	if err := appendListSubAgentsDump(b); err != nil {
		return "", err
	}
	if err := appendSwitchModeDump(b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func BuildChatToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendDocsRetrievalDump(b); err != nil {
		return "", err
	}
	if err := appendFetchWebDump(b); err != nil {
		return "", err
	}
	if err := appendWebSearchDump(b); err != nil {
		return "", err
	}
	if err := appendDeepResearchDump(b); err != nil {
		return "", err
	}
	if err := appendResearchStatusDump(b); err != nil {
		return "", err
	}
	if err := appendSwitchModeDump(b); err != nil {
		return "", err
	}
	return b.String(), nil
}
