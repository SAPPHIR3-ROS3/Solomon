package tools

func BuildBuildToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendDocsRetrievalDump(b); err != nil {
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
	if err := appendEditFileDump(b); err != nil {
		return "", err
	}
	if err := appendSubagentDump(b); err != nil {
		return "", err
	}
	if err := appendLoadSkillDump(b); err != nil {
		return "", err
	}
	if err := appendSearchSkillDump(b); err != nil {
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
