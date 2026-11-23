package sdk

func ReadBigMedia(path, intent string) (ReadResult, error) {
	raw, err := callTool("readBigMedia", map[string]string{"path": path, "intent": intent})
	if err != nil {
		return ReadResult{}, err
	}
	result, err := unmarshalToolMap(raw)
	if err != nil {
		return ReadResult{}, err
	}
	return parseReadResult(result), nil
}
