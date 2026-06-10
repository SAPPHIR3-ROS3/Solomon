package sdk

func ReplaceInFileResult(path, oldString, newString, intent string) (EditResult, error) {
	return editCall(path, oldString, newString, intent, false, "")
}

func WriteFileResult(path, content, intent string) (EditResult, error) {
	return editCall(path, "", content, intent, false, "")
}

func DeleteFileResult(path, intent string) (EditResult, error) {
	return editCall(path, "", "", intent, true, "")
}

func RenameFileResult(path, renameTo, intent string) (EditResult, error) {
	return editCall(path, "", "", intent, false, renameTo)
}

func EditFileResult(path, oldString, newString, intent string, delete bool) (EditResult, error) {
	return editCall(path, oldString, newString, intent, delete, "")
}

func ReplaceInFile(path, oldString, newString, intent string) error {
	r, err := ReplaceInFileResult(path, oldString, newString, intent)
	if err != nil {
		return err
	}
	return editErr(r)
}

func WriteFile(path, content, intent string) error {
	r, err := WriteFileResult(path, content, intent)
	if err != nil {
		return err
	}
	return editErr(r)
}

func DeleteFile(path, intent string) error {
	r, err := DeleteFileResult(path, intent)
	if err != nil {
		return err
	}
	return editErr(r)
}

func RenameFile(path, renameTo, intent string) error {
	r, err := RenameFileResult(path, renameTo, intent)
	if err != nil {
		return err
	}
	return editErr(r)
}

func EditFile(path, oldString, newString, intent string, delete bool) error {
	r, err := EditFileResult(path, oldString, newString, intent, delete)
	if err != nil {
		return err
	}
	return editErr(r)
}

func editCall(path, oldString, newString, intent string, delete bool, renameTo string) (EditResult, error) {
	args := map[string]any{
		"path": path, "oldString": oldString, "newString": newString,
		"intent": intent, "delete": delete,
	}
	if renameTo != "" {
		args["renameTo"] = renameTo
	}
	raw, err := callTool("editFile", args)
	if err != nil {
		return EditResult{}, err
	}
	return parseEditResult(raw)
}
