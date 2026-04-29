package skills

import (
	"bytes"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseSkillFrontMatter(mdPath string) (map[string]any, error) {
	b, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(b, []byte("---")) {
		return map[string]any{}, nil
	}
	rest := b[3:]
	idx := bytes.Index(rest, []byte("---"))
	if idx < 0 {
		return map[string]any{}, nil
	}
	yamlBytes := bytes.TrimSpace(rest[:idx])
	var fm map[string]any
	if len(yamlBytes) > 0 {
		if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
			return nil, err
		}
	}
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, nil
}

func DisplayNameFromFrontMatter(fm map[string]any, fallback string) string {
	if fm != nil {
		for _, k := range []string{"name", "title"} {
			if v, ok := fm[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return fallback
}
