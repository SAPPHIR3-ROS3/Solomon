package tools

func BuildPlanToolDump() (string, error) {
	b := &dumpBuilder{}
	if err := appendCreatePlanDump(b); err != nil {
		return "", err
	}
	if err := appendEditPlanDump(b); err != nil {
		return "", err
	}
	if err := appendBuildPlanDump(b); err != nil {
		return "", err
	}
	return b.String(), nil
}
