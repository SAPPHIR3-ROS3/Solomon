package tools

func BuildPlanToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendDocsRetrievalDump(b); err != nil {
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
	return b.String(), nil
}
