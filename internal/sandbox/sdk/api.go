package sdk

import (
	"encoding/json"
	"fmt"
)

func ReadFile(path string) (string, error) {
	raw, err := callTool("readFile", map[string]any{"path": path})
	if err != nil {
		return "", err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	c, _ := m["content"].(string)
	return c, nil
}

func EditFile(path, oldString, newString, intent string, delete bool) error {
	_, err := callTool("editFile", map[string]any{
		"path": path, "oldString": oldString, "newString": newString,
		"intent": intent, "delete": delete,
	})
	return err
}

func Find(pattern string, files bool) (string, error) {
	raw, err := callTool("find", map[string]any{"pattern": pattern, "files": files})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func Shell(command, intent string) (string, error) {
	raw, err := callTool("shell", map[string]any{"command": command, "intent": intent})
	if err != nil {
		return "", err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	out, _ := m["output"].(string)
	if code, ok := m["exit"].(float64); ok && code != 0 {
		return out, fmt.Errorf("shell exit %v: %s", code, out)
	}
	return out, nil
}

func WebSearch(query string) (string, error) {
	raw, err := callTool("webSearch", map[string]any{"query": query})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func FetchWeb(url string) (string, error) {
	raw, err := callTool("fetchWeb", map[string]any{"url": url})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func DocsRetrieval(query string) (string, error) {
	raw, err := callTool("docsRetrieval", map[string]any{"query": query})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func Print(v ...any) {
	fmt.Print(v...)
}

func Println(v ...any) {
	fmt.Println(v...)
}
