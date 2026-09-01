package sdk

func ReadFile(path, intent string) (string, error) {
	r, err := ReadFileInfo(path, intent)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileLines(path string, start, end int, intent string) (string, error) {
	r, err := ReadFileLinesInfo(path, start, end, intent)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileFromLine(path string, start int, intent string) (string, error) {
	r, err := ReadFileFromLineInfo(path, start, intent)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileUntilLine(path string, end int, intent string) (string, error) {
	r, err := ReadFileUntilLineInfo(path, end, intent)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func ReadFileInfo(path, intent string) (ReadResult, error) {
	return readFileCall(path, nil, nil, intent)
}

func ReadFileLinesInfo(path string, start, end int, intent string) (ReadResult, error) {
	if start < 1 {
		start = 1
	}
	s := start
	if end >= start {
		e := end
		return readFileCall(path, &s, &e, intent)
	}
	return readFileFromLine(path, s, intent)
}

func ReadFileFromLineInfo(path string, start int, intent string) (ReadResult, error) {
	return readFileFromLine(path, start, intent)
}

func ReadFileUntilLineInfo(path string, end int, intent string) (ReadResult, error) {
	return readFileUntilLine(path, end, intent)
}
