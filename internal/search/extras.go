package search

import "fmt"

func extrasString(ex map[string]any, key string) (string, error) {
	if ex == nil {
		return "", fmt.Errorf("extras.%s missing", key)
	}
	raw, ok := ex[key]
	if !ok {
		return "", fmt.Errorf("extras.%s missing", key)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("extras.%s must be a string", key)
	}
	return s, nil
}

func extrasOptionalString(ex map[string]any, key string) string {
	if ex == nil {
		return ""
	}
	raw, ok := ex[key]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return s
}
