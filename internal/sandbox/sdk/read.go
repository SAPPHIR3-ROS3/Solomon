package sdk

func ReadFile(path string) (string, error) {
	r, err := ReadFileInfo(path)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileLines(path string, start, end int) (string, error) {
	r, err := ReadFileLinesInfo(path, start, end)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileFromLine(path string, start int) (string, error) {
	r, err := ReadFileFromLineInfo(path, start)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileUntilLine(path string, end int) (string, error) {
	r, err := ReadFileUntilLineInfo(path, end)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileInfo(path string) (ReadResult, error) {
	return readFileCall(path, nil, nil)
}

func ReadFileLinesInfo(path string, start, end int) (ReadResult, error) {
	if start < 1 {
		start = 1
	}
	s := start
	if end >= start {
		e := end
		return readFileCall(path, &s, &e)
	}
	return readFileFromLine(path, s)
}

func ReadFileFromLineInfo(path string, start int) (ReadResult, error) {
	return readFileFromLine(path, start)
}

func ReadFileUntilLineInfo(path string, end int) (ReadResult, error) {
	return readFileUntilLine(path, end)
}
