package sdk

import "fmt"

type EditRange struct {
	StartLine int
	EndLine   int
}

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

func EditFileResult(path, oldString, newString, intent string, delete bool, lines ...EditRange) (EditResult, error) {
	return editCall(path, oldString, newString, intent, delete, "", lines...)
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

func EditFile(path, oldString, newString, intent string, delete bool, lines ...EditRange) error {
	r, err := EditFileResult(path, oldString, newString, intent, delete, lines...)
	if err != nil {
		return err
	}
	return editErr(r)
}

func editCall(path, oldString, newString, intent string, delete bool, renameTo string, lines ...EditRange) (EditResult, error) {
	args := map[string]any{
		"path": path, "oldString": oldString, "newString": newString,
		"intent": intent, "delete": delete,
	}
	if len(lines) > 1 {
		return EditResult{}, fmt.Errorf("editFile accepts at most one optional line range")
	}
	if len(lines) == 1 {
		selected := lines[0]
		if selected.StartLine == 0 && selected.EndLine == 0 {
			return EditResult{}, fmt.Errorf("editFile range requires startLine or endLine")
		}
		if selected.StartLine != 0 {
			args["startLine"] = selected.StartLine
		}
		if selected.EndLine != 0 {
			args["endLine"] = selected.EndLine
		}
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
